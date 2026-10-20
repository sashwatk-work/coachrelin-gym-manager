package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/auth"
	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/store"
)

type memberRequest struct {
	FullName         string  `json:"full_name"`
	Email            *string `json:"email"`
	Phone            *string `json:"phone"`
	DateOfBirth      *string `json:"date_of_birth"`
	Gender           *string `json:"gender"`
	JoinedAt         string  `json:"joined_at"`
	Status           string  `json:"status"`
	EmergencyContact *string `json:"emergency_contact"`
	Notes            string  `json:"notes"`
}

func (req *memberRequest) toInput() (store.MemberInput, error) {
	v := newValidator()
	in := store.MemberInput{
		FullName:         v.require("full_name", req.FullName),
		Email:            strPtr(req.Email),
		Phone:            strPtr(req.Phone),
		Gender:           strPtr(req.Gender),
		EmergencyContact: strPtr(req.EmergencyContact),
		Notes:            strings.TrimSpace(req.Notes),
		Status:           req.Status,
	}
	if in.Email != nil && !strings.Contains(*in.Email, "@") {
		v.add("email", "must be a valid email")
	}
	if in.Status == "" {
		in.Status = "active"
	}
	v.oneOf("status", in.Status, "active", "inactive", "frozen")
	if req.JoinedAt == "" {
		in.JoinedAt = store.DateOf(time.Now())
	} else if d, err := store.ParseDate(req.JoinedAt); err != nil {
		v.add("joined_at", "must be YYYY-MM-DD")
	} else {
		in.JoinedAt = d
	}
	if req.DateOfBirth != nil && strings.TrimSpace(*req.DateOfBirth) != "" {
		d, err := store.ParseDate(*req.DateOfBirth)
		if err != nil {
			v.add("date_of_birth", "must be YYYY-MM-DD")
		} else {
			in.DateOfBirth = &d
		}
	}
	return in, v.err()
}

func (s *Server) handleListMembers(w http.ResponseWriter, r *http.Request) {
	if err := requireLinkedTrainer(r); err != nil {
		s.respondErr(w, r, err)
		return
	}
	p := store.MemberListParams{ListParams: listParams(r), Membership: r.URL.Query().Get("membership"), TrainerID: scopedTrainerID(r)}
	var page store.Page[store.Member]
	err := s.tenantTx(r, func(tx pgx.Tx) (err error) {
		page, err = store.ListMembers(r.Context(), tx, p)
		return
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) handleCreateMember(w http.ResponseWriter, r *http.Request) {
	var req memberRequest
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
	var m store.Member
	err = s.tenantTx(r, func(tx pgx.Tx) (err error) {
		m, err = store.CreateMember(r.Context(), tx, sess.TenantID, in)
		return
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

// memberDetail bundles a member with its history for the detail page.
type memberDetail struct {
	store.Member
	Subscriptions []store.Subscription `json:"subscriptions"`
	Packages      []store.PTPackage    `json:"packages"`
	Payments      []store.Payment      `json:"payments"`
	Sessions      []store.PTSession    `json:"sessions"`
}

func (s *Server) handleGetMember(w http.ResponseWriter, r *http.Request) {
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
	var d memberDetail
	err = s.tenantTx(r, func(tx pgx.Tx) error {
		m, err := store.GetMember(r.Context(), tx, id)
		if err != nil {
			return err
		}
		if tid != nil {
			ok, err := store.MemberHasTrainer(r.Context(), tx, id, *tid)
			if err != nil {
				return err
			}
			if !ok {
				return store.ErrNotFound
			}
		}
		d.Member = m
		if d.Subscriptions, err = store.ListMemberSubscriptions(r.Context(), tx, id); err != nil {
			return err
		}
		if d.Packages, err = store.ListMemberPackages(r.Context(), tx, id); err != nil {
			return err
		}
		if d.Payments, err = store.ListMemberPayments(r.Context(), tx, id); err != nil {
			return err
		}
		sessions, err := store.ListSessions(r.Context(), tx, store.SessionListParams{
			ListParams: store.ListParams{PageSize: 100}, MemberID: &id, TrainerID: tid, Desc: true,
		})
		if err != nil {
			return err
		}
		d.Sessions = sessions.Items
		if tid != nil {
			filtered := d.Packages[:0]
			for _, pk := range d.Packages {
				if pk.TrainerID == *tid {
					filtered = append(filtered, pk)
				}
			}
			d.Packages = filtered
			d.Payments = []store.Payment{}
		}
		return nil
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) handleUpdateMember(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	var req memberRequest
	if err := decode(r, &req); err != nil {
		s.respondErr(w, r, err)
		return
	}
	in, err := req.toInput()
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	var m store.Member
	err = s.tenantTx(r, func(tx pgx.Tx) (err error) {
		m, err = store.UpdateMember(r.Context(), tx, id, in)
		return
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleDeleteMember(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	err = s.tenantTx(r, func(tx pgx.Tx) error {
		return store.DeleteMember(r.Context(), tx, id)
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// ---------------------------------------------------------------------------
// Subscriptions
// ---------------------------------------------------------------------------

type subscriptionRequest struct {
	PlanID    string   `json:"plan_id"`
	StartDate string   `json:"start_date"`
	EndDate   string   `json:"end_date"` // optional; defaults to start + plan duration
	Price     *float64 `json:"price"`    // optional; defaults to plan price
}

func (s *Server) handleCreateSubscription(w http.ResponseWriter, r *http.Request) {
	memberID, err := urlUUID(r, "id")
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	var req subscriptionRequest
	if err := decode(r, &req); err != nil {
		s.respondErr(w, r, err)
		return
	}
	v := newValidator()
	planID, perr := uuid.Parse(req.PlanID)
	if perr != nil {
		v.add("plan_id", "is required")
	}
	start := store.DateOf(time.Now())
	if req.StartDate != "" {
		if d, err := store.ParseDate(req.StartDate); err != nil {
			v.add("start_date", "must be YYYY-MM-DD")
		} else {
			start = d
		}
	}
	var end *store.Date
	if req.EndDate != "" {
		if d, err := store.ParseDate(req.EndDate); err != nil {
			v.add("end_date", "must be YYYY-MM-DD")
		} else {
			end = &d
		}
	}
	if req.Price != nil && *req.Price < 0 {
		v.add("price", "must be >= 0")
	}
	if err := v.err(); err != nil {
		s.respondErr(w, r, err)
		return
	}

	sess, _ := auth.FromContext(r.Context())
	var sub store.Subscription
	err = s.tenantTx(r, func(tx pgx.Tx) error {
		if _, err := store.GetMember(r.Context(), tx, memberID); err != nil {
			return err
		}
		plan, err := store.GetPlan(r.Context(), tx, planID)
		if err != nil {
			return &ValidationError{Fields: map[string]string{"plan_id": "plan not found"}}
		}
		in := store.SubscriptionInput{PlanID: plan.ID, StartDate: start, Price: plan.Price}
		if end != nil {
			in.EndDate = *end
		} else {
			in.EndDate = start.AddDays(plan.DurationDays - 1)
		}
		if in.EndDate.Before(in.StartDate.Time) {
			return &ValidationError{Fields: map[string]string{"end_date": "must be on or after start date"}}
		}
		if req.Price != nil {
			in.Price = *req.Price
		}
		sub, err = store.CreateSubscription(r.Context(), tx, sess.TenantID, memberID, in)
		return err
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, sub)
}

type subscriptionPatch struct {
	Status string `json:"status"` // active | cancelled
}

func (s *Server) handleUpdateSubscription(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	var req subscriptionPatch
	if err := decode(r, &req); err != nil {
		s.respondErr(w, r, err)
		return
	}
	v := newValidator()
	v.oneOf("status", req.Status, "active", "cancelled")
	if err := v.err(); err != nil {
		s.respondErr(w, r, err)
		return
	}
	var sub store.Subscription
	err = s.tenantTx(r, func(tx pgx.Tx) (err error) {
		sub, err = store.SetSubscriptionStatus(r.Context(), tx, id, req.Status)
		return
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, sub)
}
