package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/auth"
	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/store"
)

// ---------------------------------------------------------------------------
// PT plans (reusable package templates)
// ---------------------------------------------------------------------------

type ptPlanRequest struct {
	Name         string  `json:"name"`
	Sessions     int     `json:"sessions"`
	ValidityDays *int    `json:"validity_days"` // nil / omitted = never expires
	Price        float64 `json:"price"`
	Description  string  `json:"description"`
	Active       *bool   `json:"active"`
}

func (req *ptPlanRequest) toInput() (store.PTPlanInput, error) {
	v := newValidator()
	in := store.PTPlanInput{
		Name:         v.require("name", req.Name),
		Sessions:     req.Sessions,
		ValidityDays: req.ValidityDays,
		Price:        req.Price,
		Description:  strings.TrimSpace(req.Description),
		Active:       true,
	}
	if req.Active != nil {
		in.Active = *req.Active
	}
	if in.Sessions <= 0 {
		v.add("sessions", "must be greater than 0")
	}
	if in.ValidityDays != nil && *in.ValidityDays <= 0 {
		v.add("validity_days", "must be greater than 0")
	}
	if in.Price < 0 {
		v.add("price", "must be >= 0")
	}
	return in, v.err()
}

func (s *Server) handleListPTPlans(w http.ResponseWriter, r *http.Request) {
	activeOnly := r.URL.Query().Get("active") == "true"
	var plans []store.PTPlan
	err := s.tenantTx(r, func(tx pgx.Tx) (err error) {
		plans, err = store.ListPTPlans(r.Context(), tx, activeOnly)
		return
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": plans})
}

func (s *Server) handleCreatePTPlan(w http.ResponseWriter, r *http.Request) {
	var req ptPlanRequest
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
	var p store.PTPlan
	err = s.tenantTx(r, func(tx pgx.Tx) (err error) {
		p, err = store.CreatePTPlan(r.Context(), tx, sess.TenantID, in)
		return
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) handleUpdatePTPlan(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	var req ptPlanRequest
	if err := decode(r, &req); err != nil {
		s.respondErr(w, r, err)
		return
	}
	in, err := req.toInput()
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	var p store.PTPlan
	err = s.tenantTx(r, func(tx pgx.Tx) (err error) {
		p, err = store.UpdatePTPlan(r.Context(), tx, id, in)
		return
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleDeletePTPlan(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	err = s.tenantTx(r, func(tx pgx.Tx) error {
		return store.DeletePTPlan(r.Context(), tx, id)
	})
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			writeError(w, http.StatusConflict, "plan is in use by packages; mark it inactive instead", nil)
			return
		}
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// ---------------------------------------------------------------------------
// Packages
// ---------------------------------------------------------------------------

// packageRequest creates or updates a PT package. The simplest form is just
// {member_id, trainer_id, pt_plan_id}: name, sessions, price and expiry are
// taken from the plan. Any of those may be supplied to customise the package.
// Without pt_plan_id it is a custom package and name/sessions/price are
// required. Price is a pointer so an explicit 0 can be told apart from
// "use the plan's price".
type packageRequest struct {
	MemberID      string   `json:"member_id"`
	TrainerID     string   `json:"trainer_id"`
	PTPlanID      *string  `json:"pt_plan_id"`
	Name          string   `json:"name"`
	TotalSessions int      `json:"total_sessions"`
	Price         *float64 `json:"price"`
	PurchasedAt   string   `json:"purchased_at"`
	ExpiresAt     *string  `json:"expires_at"`
	Status        string   `json:"status"`
}

// planID parses the optional pt_plan_id.
func (req *packageRequest) planID() (*uuid.UUID, error) {
	if req.PTPlanID == nil || strings.TrimSpace(*req.PTPlanID) == "" {
		return nil, nil
	}
	id, err := uuid.Parse(*req.PTPlanID)
	if err != nil {
		return nil, &ValidationError{Fields: map[string]string{"pt_plan_id": "is invalid"}}
	}
	return &id, nil
}

// toInput builds the store input, applying plan defaults first if a plan was
// given, then validating the result.
func (req *packageRequest) toInput(requireMember bool, plan *store.PTPlan) (store.PackageInput, error) {
	v := newValidator()
	in := store.PackageInput{
		Name:          strings.TrimSpace(req.Name),
		TotalSessions: req.TotalSessions,
		Status:        req.Status,
		PurchasedAt:   store.DateOf(time.Now()),
	}
	if req.Price != nil {
		in.Price = *req.Price
	}
	if requireMember {
		if id, err := uuid.Parse(req.MemberID); err != nil {
			v.add("member_id", "is required")
		} else {
			in.MemberID = id
		}
	}
	if id, err := uuid.Parse(req.TrainerID); err != nil {
		v.add("trainer_id", "is required")
	} else {
		in.TrainerID = id
	}
	if req.PurchasedAt != "" {
		if d, err := store.ParseDate(req.PurchasedAt); err != nil {
			v.add("purchased_at", "must be YYYY-MM-DD")
		} else {
			in.PurchasedAt = d
		}
	}
	if req.ExpiresAt != nil {
		// The field was sent: empty means "no expiry" (do not take the plan's
		// validity), a date is an explicit override.
		in.SkipPlanExpiry = true
		if strings.TrimSpace(*req.ExpiresAt) != "" {
			if d, err := store.ParseDate(*req.ExpiresAt); err != nil {
				v.add("expires_at", "must be YYYY-MM-DD")
			} else {
				in.ExpiresAt = &d
			}
		}
	}

	if plan != nil {
		in.ApplyPlan(*plan, req.Price)
	} else if req.Price == nil {
		v.add("price", "is required")
	}

	if in.Name == "" {
		v.add("name", "is required")
	}
	if in.TotalSessions <= 0 {
		v.add("total_sessions", "must be greater than 0")
	}
	if in.Price < 0 {
		v.add("price", "must be >= 0")
	}
	if in.ExpiresAt != nil && in.ExpiresAt.Before(in.PurchasedAt.Time) {
		v.add("expires_at", "must be on or after the purchase date")
	}
	if in.Status == "" {
		in.Status = "active"
	}
	v.oneOf("status", in.Status, "active", "completed", "expired", "cancelled")
	return in, v.err()
}

// loadPlan fetches the plan referenced by the request, if any. Inactive
// plans can still be referenced when editing an existing package.
func (s *Server) loadPlan(r *http.Request, tx pgx.Tx, req *packageRequest, allowInactive bool) (*store.PTPlan, error) {
	id, err := req.planID()
	if err != nil || id == nil {
		return nil, err
	}
	p, err := store.GetPTPlan(r.Context(), tx, *id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, &ValidationError{Fields: map[string]string{"pt_plan_id": "plan not found"}}
		}
		return nil, err
	}
	if !p.Active && !allowInactive {
		return nil, &ValidationError{Fields: map[string]string{"pt_plan_id": "plan is inactive"}}
	}
	return &p, nil
}

func (s *Server) handleListPackages(w http.ResponseWriter, r *http.Request) {
	memberID, err := optUUID(r, "member_id")
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	trainerID, err := optUUID(r, "trainer_id")
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	if err := requireLinkedTrainer(r); err != nil {
		s.respondErr(w, r, err)
		return
	}
	if scoped := scopedTrainerID(r); scoped != nil {
		trainerID = scoped
	}
	p := store.PackageListParams{ListParams: listParams(r), MemberID: memberID, TrainerID: trainerID}
	var page store.Page[store.PTPackage]
	err = s.tenantTx(r, func(tx pgx.Tx) (err error) {
		page, err = store.ListPackages(r.Context(), tx, p)
		return
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) handleCreatePackage(w http.ResponseWriter, r *http.Request) {
	var req packageRequest
	if err := decode(r, &req); err != nil {
		s.respondErr(w, r, err)
		return
	}
	sess, _ := auth.FromContext(r.Context())
	var pk store.PTPackage
	err := s.tenantTx(r, func(tx pgx.Tx) error {
		plan, err := s.loadPlan(r, tx, &req, false)
		if err != nil {
			return err
		}
		in, err := req.toInput(true, plan)
		if err != nil {
			return err
		}
		pk, err = store.CreatePackage(r.Context(), tx, sess.TenantID, in)
		return err
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "member or trainer not found", nil)
			return
		}
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, pk)
}

type packageDetail struct {
	store.PTPackage
	Sessions []store.PTSession `json:"sessions"`
	Payments []store.Payment   `json:"payments"`
}

func (s *Server) handleGetPackage(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	if err := requireLinkedTrainer(r); err != nil {
		s.respondErr(w, r, err)
		return
	}
	tid := scopedTrainerID(r)
	var d packageDetail
	err = s.tenantTx(r, func(tx pgx.Tx) error {
		pk, err := store.GetPackage(r.Context(), tx, id)
		if err != nil {
			return err
		}
		if tid != nil && pk.TrainerID != *tid {
			return store.ErrNotFound
		}
		d.PTPackage = pk
		sessions, err := store.ListSessions(r.Context(), tx, store.SessionListParams{
			ListParams: store.ListParams{PageSize: 200}, PackageID: &id, Desc: true,
		})
		if err != nil {
			return err
		}
		d.Sessions = sessions.Items
		all, err := store.ListMemberPayments(r.Context(), tx, pk.MemberID)
		if err != nil {
			return err
		}
		d.Payments = []store.Payment{}
		if tid == nil {
			for _, p := range all {
				if p.PTPackageID != nil && *p.PTPackageID == id {
					d.Payments = append(d.Payments, p)
				}
			}
		}
		return nil
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) handleUpdatePackage(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	var req packageRequest
	if err := decode(r, &req); err != nil {
		s.respondErr(w, r, err)
		return
	}
	var pk store.PTPackage
	err = s.tenantTx(r, func(tx pgx.Tx) error {
		plan, err := s.loadPlan(r, tx, &req, true)
		if err != nil {
			return err
		}
		in, err := req.toInput(false, plan)
		if err != nil {
			return err
		}
		pk, err = store.UpdatePackage(r.Context(), tx, id, in)
		return err
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, pk)
}

func (s *Server) handleDeletePackage(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	err = s.tenantTx(r, func(tx pgx.Tx) error {
		return store.DeletePackage(r.Context(), tx, id)
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// ---------------------------------------------------------------------------
// Sessions
// ---------------------------------------------------------------------------

var sessionStatuses = []string{"scheduled", "completed", "cancelled", "no_show"}

type sessionRequest struct {
	PackageID       string  `json:"package_id"`
	TrainerID       *string `json:"trainer_id"`
	ScheduledAt     string  `json:"scheduled_at"` // RFC3339
	DurationMinutes int     `json:"duration_minutes"`
	Status          string  `json:"status"`
	Notes           string  `json:"notes"`
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	memberID, err := optUUID(r, "member_id")
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	trainerID, err := optUUID(r, "trainer_id")
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	packageID, err := optUUID(r, "package_id")
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	from, err := optDate(r, "from")
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	to, err := optDate(r, "to")
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	if err := requireLinkedTrainer(r); err != nil {
		s.respondErr(w, r, err)
		return
	}
	if scoped := scopedTrainerID(r); scoped != nil {
		trainerID = scoped
	}
	p := store.SessionListParams{
		ListParams: listParams(r),
		MemberID:   memberID,
		TrainerID:  trainerID,
		PackageID:  packageID,
		From:       from,
		To:         to,
		Desc:       r.URL.Query().Get("order") == "desc",
	}
	var page store.Page[store.PTSession]
	err = s.tenantTx(r, func(tx pgx.Tx) (err error) {
		page, err = store.ListSessions(r.Context(), tx, p)
		return
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req sessionRequest
	if err := decode(r, &req); err != nil {
		s.respondErr(w, r, err)
		return
	}
	v := newValidator()
	in := store.SessionInput{DurationMinutes: req.DurationMinutes, Status: req.Status, Notes: strings.TrimSpace(req.Notes)}
	if id, err := uuid.Parse(req.PackageID); err != nil {
		v.add("package_id", "is required")
	} else {
		in.PackageID = id
	}
	if req.TrainerID != nil && *req.TrainerID != "" {
		if id, err := uuid.Parse(*req.TrainerID); err != nil {
			v.add("trainer_id", "is invalid")
		} else {
			in.TrainerID = &id
		}
	}
	if t, err := parseFlexibleTime(req.ScheduledAt); err != nil || req.ScheduledAt == "" {
		v.add("scheduled_at", "is required (RFC3339)")
	} else {
		in.ScheduledAt = t
	}
	if in.Status != "" {
		v.oneOf("status", in.Status, sessionStatuses...)
	}
	if in.DurationMinutes < 0 || in.DurationMinutes > 24*60 {
		v.add("duration_minutes", "must be between 1 and 1440")
	}
	if err := v.err(); err != nil {
		s.respondErr(w, r, err)
		return
	}

	if err := requireRoles(r, store.RoleOwner, store.RoleReceptionist, store.RoleTrainer); err != nil {
		s.respondErr(w, r, err)
		return
	}
	if err := requireLinkedTrainer(r); err != nil {
		s.respondErr(w, r, err)
		return
	}
	sess, _ := auth.FromContext(r.Context())
	if trainerOnly(sess) {
		in.TrainerID = &sess.TrainerID
	}
	var out store.PTSession
	err := s.tenantTx(r, func(tx pgx.Tx) (err error) {
		if trainerOnly(sess) {
			pk, err := store.GetPackage(r.Context(), tx, in.PackageID)
			if err != nil {
				return err
			}
			if pk.TrainerID != sess.TrainerID {
				return store.ErrNotFound
			}
		}
		out, err = store.CreateSession(r.Context(), tx, sess.TenantID, in)
		return
	})
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNoSessionsLeft):
			writeError(w, http.StatusConflict, "this package has no sessions left", nil)
		case errors.Is(err, store.ErrPackageNotActive):
			writeError(w, http.StatusConflict, "this package is not active", nil)
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusBadRequest, "package or trainer not found", nil)
		default:
			s.respondErr(w, r, err)
		}
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleUpdateSession(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	var req sessionRequest
	if err := decode(r, &req); err != nil {
		s.respondErr(w, r, err)
		return
	}
	v := newValidator()
	var trainerID uuid.UUID
	if req.TrainerID == nil {
		v.add("trainer_id", "is required")
	} else if tid, err := uuid.Parse(*req.TrainerID); err != nil {
		v.add("trainer_id", "is invalid")
	} else {
		trainerID = tid
	}
	scheduledAt, terr := parseFlexibleTime(req.ScheduledAt)
	if terr != nil || req.ScheduledAt == "" {
		v.add("scheduled_at", "is required (RFC3339)")
	}
	if req.DurationMinutes <= 0 || req.DurationMinutes > 24*60 {
		v.add("duration_minutes", "must be between 1 and 1440")
	}
	if req.Status == "" {
		req.Status = "scheduled"
	}
	v.oneOf("status", req.Status, sessionStatuses...)
	if err := v.err(); err != nil {
		s.respondErr(w, r, err)
		return
	}
	if err := requireLinkedTrainer(r); err != nil {
		s.respondErr(w, r, err)
		return
	}
	sess := sessionOf(r)
	if trainerOnly(sess) {
		trainerID = sess.TrainerID
	}
	var out store.PTSession
	err = s.tenantTx(r, func(tx pgx.Tx) (err error) {
		if trainerOnly(sess) {
			cur, err := store.GetSession(r.Context(), tx, id)
			if err != nil {
				return err
			}
			if cur.TrainerID != sess.TrainerID {
				return store.ErrNotFound
			}
		}
		out, err = store.UpdateSession(r.Context(), tx, id, trainerID, scheduledAt, req.DurationMinutes, req.Status, strings.TrimSpace(req.Notes))
		return
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

type sessionStatusPatch struct {
	Status string  `json:"status"`
	Notes  *string `json:"notes"`
}

func (s *Server) handleUpdateSessionStatus(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	var req sessionStatusPatch
	if err := decode(r, &req); err != nil {
		s.respondErr(w, r, err)
		return
	}
	v := newValidator()
	v.oneOf("status", req.Status, sessionStatuses...)
	if err := v.err(); err != nil {
		s.respondErr(w, r, err)
		return
	}
	if err := requireLinkedTrainer(r); err != nil {
		s.respondErr(w, r, err)
		return
	}
	sess := sessionOf(r)
	var out store.PTSession
	err = s.tenantTx(r, func(tx pgx.Tx) (err error) {
		if trainerOnly(sess) {
			cur, err := store.GetSession(r.Context(), tx, id)
			if err != nil {
				return err
			}
			if cur.TrainerID != sess.TrainerID {
				return store.ErrNotFound
			}
		}
		out, err = store.SetSessionStatus(r.Context(), tx, id, req.Status, req.Notes)
		return
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	err = s.tenantTx(r, func(tx pgx.Tx) error {
		return store.DeleteSession(r.Context(), tx, id)
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}
