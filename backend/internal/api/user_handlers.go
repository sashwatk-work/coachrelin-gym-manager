package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/auth"
	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/store"
)

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	sess := sessionOf(r)
	var users []store.User
	err := s.tenantTx(r, func(tx pgx.Tx) (err error) {
		users, err = store.ListUsers(r.Context(), tx, sess.TenantID)
		return
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": users})
}

type userRequest struct {
	FullName  string   `json:"full_name"`
	Email     string   `json:"email"`
	Password  string   `json:"password"`
	Roles     []string `json:"roles"`
	Role      string   `json:"role"` // legacy single-role clients
	TrainerID *string  `json:"trainer_id"`
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req userRequest
	if err := decode(r, &req); err != nil {
		s.respondErr(w, r, err)
		return
	}
	v := newValidator()
	req.FullName = v.require("full_name", req.FullName)
	req.Email = strings.ToLower(v.require("email", req.Email))
	if req.Email != "" && !strings.Contains(req.Email, "@") {
		v.add("email", "must be a valid email")
	}
	if len(req.Password) < 8 {
		v.add("password", "must be at least 8 characters")
	}
	roles := decodeRoles(v, req.Roles, req.Role)
	trainerID, _ := parseTrainerID(v, roles, req.TrainerID)
	if err := v.err(); err != nil {
		s.respondErr(w, r, err)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		s.respondErr(w, r, err)
		return
	}

	sess := sessionOf(r)
	var u store.User
	err = s.tenantTx(r, func(tx pgx.Tx) error {
		if trainerID != nil {
			if _, err := store.GetTrainer(r.Context(), tx, *trainerID); err != nil {
				if errors.Is(err, store.ErrNotFound) {
					return &ValidationError{Fields: map[string]string{"trainer_id": "trainer not found"}}
				}
				return err
			}
			users, err := store.ListUsers(r.Context(), tx, sess.TenantID)
			if err != nil {
				return err
			}
			for _, existing := range users {
				if existing.TrainerID != nil && *existing.TrainerID == *trainerID {
					return &ValidationError{Fields: map[string]string{"trainer_id": "this trainer already has a login"}}
				}
			}
		}
		created, err := store.CreateUser(r.Context(), tx, sess.TenantID, req.Email, hash, req.FullName, roles, trainerID)
		if err != nil {
			if errors.Is(err, store.ErrConflict) {
				return &ValidationError{Fields: map[string]string{"email": "is already registered"}}
			}
			return err
		}
		u = created
		return nil
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

type userPatch struct {
	FullName  *string  `json:"full_name"`
	Roles     []string `json:"roles"`
	Role      *string  `json:"role"`
	Password  *string  `json:"password"`
	TrainerID *string  `json:"trainer_id"`
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	var req userPatch
	if err := decode(r, &req); err != nil {
		s.respondErr(w, r, err)
		return
	}
	sess := sessionOf(r)
	var out store.User
	err = s.tenantTx(r, func(tx pgx.Tx) error {
		cur, err := store.GetUser(r.Context(), tx, sess.TenantID, id)
		if err != nil {
			return err
		}
		in := store.UserUpdate{FullName: cur.FullName, Roles: cur.Roles, TrainerID: cur.TrainerID}
		v := newValidator()
		if req.FullName != nil {
			in.FullName = v.require("full_name", *req.FullName)
		}
		if len(req.Roles) > 0 || req.Role != nil {
			legacy := ""
			if req.Role != nil {
				legacy = *req.Role
			}
			in.Roles = decodeRoles(v, req.Roles, legacy)
		}
		if req.Password != nil && *req.Password != "" {
			if len(*req.Password) < 8 {
				v.add("password", "must be at least 8 characters")
			} else {
				hash, err := auth.HashPassword(*req.Password)
				if err != nil {
					return err
				}
				in.PasswordHash = &hash
			}
		}
		if req.TrainerID != nil || len(req.Roles) > 0 || req.Role != nil {
			var raw *string
			if req.TrainerID != nil {
				raw = req.TrainerID
			} else if containsRole(in.Roles, store.RoleTrainer) && cur.TrainerID != nil {
				s := cur.TrainerID.String()
				raw = &s
			}
			in.TrainerID, _ = parseTrainerID(v, in.Roles, raw)
		}
		if !containsRole(in.Roles, store.RoleOwner) && cur.HasRole(store.RoleOwner) {
			n, err := store.CountOwners(r.Context(), tx, sess.TenantID)
			if err != nil {
				return err
			}
			if n <= 1 {
				return errStatus(http.StatusConflict, "the gym must keep at least one owner")
			}
		}
		if err := v.err(); err != nil {
			return err
		}
		if in.TrainerID != nil {
			if _, err := store.GetTrainer(r.Context(), tx, *in.TrainerID); err != nil {
				if errors.Is(err, store.ErrNotFound) {
					return &ValidationError{Fields: map[string]string{"trainer_id": "trainer not found"}}
				}
				return err
			}
		}
		out, err = store.UpdateUser(r.Context(), tx, sess.TenantID, id, in)
		if errors.Is(err, store.ErrConflict) {
			return &ValidationError{Fields: map[string]string{"trainer_id": "this trainer already has a login"}}
		}
		return err
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	sess := sessionOf(r)
	if id == sess.UserID {
		writeError(w, http.StatusConflict, "you cannot delete your own login", nil)
		return
	}
	err = s.tenantTx(r, func(tx pgx.Tx) error {
		cur, err := store.GetUser(r.Context(), tx, sess.TenantID, id)
		if err != nil {
			return err
		}
		if cur.HasRole(store.RoleOwner) {
			n, err := store.CountOwners(r.Context(), tx, sess.TenantID)
			if err != nil {
				return err
			}
			if n <= 1 {
				return errStatus(http.StatusConflict, "the gym must keep at least one owner")
			}
		}
		return store.DeleteUser(r.Context(), tx, sess.TenantID, id)
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func decodeRoles(v *validator, roles []string, legacy string) []string {
	src := roles
	if len(src) == 0 && strings.TrimSpace(legacy) != "" {
		src = []string{legacy}
	}
	out, err := store.NormalizeRoles(src)
	if err != nil {
		v.add("roles", err.Error())
		return nil
	}
	return out
}

func containsRole(roles []string, role string) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

func parseTrainerID(v *validator, roles []string, raw *string) (*uuid.UUID, error) {
	if containsRole(roles, store.RoleTrainer) {
		if raw == nil || strings.TrimSpace(*raw) == "" {
			v.add("trainer_id", "pick the trainer this login belongs to")
			return nil, v.err()
		}
		id, err := uuid.Parse(strings.TrimSpace(*raw))
		if err != nil {
			v.add("trainer_id", "is invalid")
			return nil, v.err()
		}
		return &id, nil
	}
	if raw != nil && strings.TrimSpace(*raw) != "" {
		v.add("trainer_id", "only trainer logins are linked to a trainer")
	}
	return nil, v.err()
}
