package api

import (
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/auth"
	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/store"
)

type trainerRequest struct {
	FullName    string   `json:"full_name"`
	Email       *string  `json:"email"`
	Phone       *string  `json:"phone"`
	Specialties []string `json:"specialties"`
	Active      *bool    `json:"active"`
}

func (req *trainerRequest) toInput() (store.TrainerInput, error) {
	v := newValidator()
	in := store.TrainerInput{
		FullName: v.require("full_name", req.FullName),
		Email:    strPtr(req.Email),
		Phone:    strPtr(req.Phone),
		Active:   true,
	}
	if req.Active != nil {
		in.Active = *req.Active
	}
	if in.Email != nil && !strings.Contains(*in.Email, "@") {
		v.add("email", "must be a valid email")
	}
	in.Specialties = make([]string, 0, len(req.Specialties))
	for _, sp := range req.Specialties {
		if t := strings.TrimSpace(sp); t != "" {
			in.Specialties = append(in.Specialties, t)
		}
	}
	return in, v.err()
}

func (s *Server) handleListTrainers(w http.ResponseWriter, r *http.Request) {
	p := listParams(r)
	var page store.Page[store.Trainer]
	err := s.tenantTx(r, func(tx pgx.Tx) (err error) {
		page, err = store.ListTrainers(r.Context(), tx, p)
		return
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) handleCreateTrainer(w http.ResponseWriter, r *http.Request) {
	var req trainerRequest
	if err := decode(r, &req); err != nil {
		s.respondErr(w, r, err)
		return
	}
	in, err := req.toInput()
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	sess, _ := auth.FromContext(r.Context())
	var t store.Trainer
	err = s.tenantTx(r, func(tx pgx.Tx) (err error) {
		t, err = store.CreateTrainer(r.Context(), tx, sess.TenantID, in)
		return
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

type trainerDetail struct {
	store.Trainer
	Packages []store.PTPackage `json:"packages"`
	Sessions []store.PTSession `json:"sessions"` // upcoming
}

func (s *Server) handleGetTrainer(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	var d trainerDetail
	err = s.tenantTx(r, func(tx pgx.Tx) error {
		t, err := store.GetTrainer(r.Context(), tx, id)
		if err != nil {
			return err
		}
		d.Trainer = t
		pk, err := store.ListPackages(r.Context(), tx, store.PackageListParams{
			ListParams: store.ListParams{PageSize: 100}, TrainerID: &id,
		})
		if err != nil {
			return err
		}
		d.Packages = pk.Items
		sessions, err := store.ListSessions(r.Context(), tx, store.SessionListParams{
			ListParams: store.ListParams{PageSize: 100}, TrainerID: &id, Desc: true,
		})
		if err != nil {
			return err
		}
		d.Sessions = sessions.Items
		return nil
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) handleUpdateTrainer(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	var req trainerRequest
	if err := decode(r, &req); err != nil {
		s.respondErr(w, r, err)
		return
	}
	in, err := req.toInput()
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	var t store.Trainer
	err = s.tenantTx(r, func(tx pgx.Tx) (err error) {
		t, err = store.UpdateTrainer(r.Context(), tx, id, in)
		return
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleDeleteTrainer(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	err = s.tenantTx(r, func(tx pgx.Tx) error {
		return store.DeleteTrainer(r.Context(), tx, id)
	})
	if err != nil {
		if err == store.ErrConflict {
			writeError(w, http.StatusConflict, "trainer has packages or sessions; deactivate instead", nil)
			return
		}
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// ---------------------------------------------------------------------------
// Plans
// ---------------------------------------------------------------------------

type planRequest struct {
	Name         string  `json:"name"`
	DurationDays int     `json:"duration_days"`
	Price        float64 `json:"price"`
	Description  string  `json:"description"`
	Active       *bool   `json:"active"`
}

func (req *planRequest) toInput() (store.PlanInput, error) {
	v := newValidator()
	in := store.PlanInput{
		Name:         v.require("name", req.Name),
		DurationDays: req.DurationDays,
		Price:        req.Price,
		Description:  strings.TrimSpace(req.Description),
		Active:       true,
	}
	if req.Active != nil {
		in.Active = *req.Active
	}
	if in.DurationDays <= 0 {
		v.add("duration_days", "must be greater than 0")
	}
	if in.Price < 0 {
		v.add("price", "must be >= 0")
	}
	return in, v.err()
}

func (s *Server) handleListPlans(w http.ResponseWriter, r *http.Request) {
	activeOnly := r.URL.Query().Get("active") == "true"
	var plans []store.MembershipPlan
	err := s.tenantTx(r, func(tx pgx.Tx) (err error) {
		plans, err = store.ListPlans(r.Context(), tx, activeOnly)
		return
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": plans})
}

func (s *Server) handleCreatePlan(w http.ResponseWriter, r *http.Request) {
	var req planRequest
	if err := decode(r, &req); err != nil {
		s.respondErr(w, r, err)
		return
	}
	in, err := req.toInput()
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	sess, _ := auth.FromContext(r.Context())
	var p store.MembershipPlan
	err = s.tenantTx(r, func(tx pgx.Tx) (err error) {
		p, err = store.CreatePlan(r.Context(), tx, sess.TenantID, in)
		return
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) handleUpdatePlan(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	var req planRequest
	if err := decode(r, &req); err != nil {
		s.respondErr(w, r, err)
		return
	}
	in, err := req.toInput()
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	var p store.MembershipPlan
	err = s.tenantTx(r, func(tx pgx.Tx) (err error) {
		p, err = store.UpdatePlan(r.Context(), tx, id, in)
		return
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleDeletePlan(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	err = s.tenantTx(r, func(tx pgx.Tx) error {
		return store.DeletePlan(r.Context(), tx, id)
	})
	if err != nil {
		if err == store.ErrConflict {
			writeError(w, http.StatusConflict, "plan is in use by subscriptions; mark it inactive instead", nil)
			return
		}
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}
