package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/auth"
	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/store"
)

type authResponse struct {
	User   store.User   `json:"user"`
	Tenant store.Tenant `json:"tenant"`
}

type registerRequest struct {
	GymName  string `json:"gym_name"`
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Timezone string `json:"timezone"`
	Currency string `json:"currency"`
}

// handleRegister creates a gym and its owner in a single transaction, then
// signs the owner in. There is no public signup in the UI; this stays for
// integration tests and for adding Coach Relin Fitness locations later.
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decode(r, &req); err != nil {
		s.respondErr(w, r, err)
		return
	}
	v := newValidator()
	req.GymName = v.require("gym_name", req.GymName)
	req.FullName = v.require("full_name", req.FullName)
	req.Email = strings.ToLower(v.require("email", req.Email))
	if req.Email != "" && !strings.Contains(req.Email, "@") {
		v.add("email", "must be a valid email")
	}
	if len(req.Password) < 8 {
		v.add("password", "must be at least 8 characters")
	}
	if req.Timezone == "" {
		req.Timezone = "UTC"
	} else if _, err := time.LoadLocation(req.Timezone); err != nil {
		v.add("timezone", "must be a valid IANA timezone")
	}
	req.Currency = strings.ToUpper(strings.TrimSpace(req.Currency))
	if req.Currency == "" {
		req.Currency = "USD"
	} else if len(req.Currency) != 3 {
		v.add("currency", "must be a 3-letter ISO code")
	}
	if err := v.err(); err != nil {
		s.respondErr(w, r, err)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		s.respondErr(w, r, err)
		return
	}

	var resp authResponse
	err = s.db.WithTx(r.Context(), func(tx pgx.Tx) error {
		t, err := store.CreateTenant(r.Context(), tx, req.GymName, req.Timezone, req.Currency)
		if err != nil {
			return err
		}
		u, err := store.CreateUser(r.Context(), tx, t.ID, req.Email, hash, req.FullName, []string{store.RoleOwner}, nil)
		if err != nil {
			if errors.Is(err, store.ErrConflict) {
				return &ValidationError{Fields: map[string]string{"email": "is already registered"}}
			}
			return err
		}
		resp = authResponse{User: u, Tenant: t}
		return nil
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}

	if err := s.issueSession(w, resp.User); err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decode(r, &req); err != nil {
		s.respondErr(w, r, err)
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required", nil)
		return
	}

	u, err := store.GetUserByEmail(r.Context(), s.db.Pool, req.Email)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.respondErr(w, r, err)
		return
	}
	// Constant-time-ish: always run bcrypt so a missing user is not faster.
	hash := "$2a$10$invalidinvalidinvalidinvalidinvalidinvalidinvalidinvalid"
	if err == nil {
		hash = u.PasswordHash
	}
	if !auth.CheckPassword(hash, req.Password) || err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password", nil)
		return
	}

	t, err := store.GetTenant(r.Context(), s.db.Pool, u.TenantID)
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	if err := s.issueSession(w, u); err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, authResponse{User: u, Tenant: t})
}

func (s *Server) handleLogout(w http.ResponseWriter, _ *http.Request) {
	auth.ClearSessionCookie(w, s.cfg.CookieSecure)
	writeJSON(w, http.StatusNoContent, nil)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	u, err := store.GetUser(r.Context(), s.db.Pool, sess.TenantID, sess.UserID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			auth.ClearSessionCookie(w, s.cfg.CookieSecure)
			writeError(w, http.StatusUnauthorized, "unauthenticated", nil)
			return
		}
		s.respondErr(w, r, err)
		return
	}
	t, err := store.GetTenant(r.Context(), s.db.Pool, u.TenantID)
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, authResponse{User: u, Tenant: t})
}

func (s *Server) issueSession(w http.ResponseWriter, u store.User) error {
	tok, err := s.tokens.Issue(u.ID, u.TenantID, u.Roles, trainerIDOfUser(u))
	if err != nil {
		return err
	}
	auth.SetSessionCookie(w, tok, s.tokens.TTL(), s.cfg.CookieSecure)
	return nil
}
