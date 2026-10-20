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

var paymentMethods = []string{"cash", "card", "upi", "bank_transfer", "other"}

func (s *Server) handleListPayments(w http.ResponseWriter, r *http.Request) {
	memberID, err := optUUID(r, "member_id")
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
	p := store.PaymentListParams{
		ListParams: listParams(r),
		MemberID:   memberID,
		Method:     r.URL.Query().Get("method"),
		From:       from,
		To:         to,
	}
	var page store.PaymentPage
	err = s.tenantTx(r, func(tx pgx.Tx) (err error) {
		page, err = store.ListPayments(r.Context(), tx, p)
		return
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

type paymentRequest struct {
	MemberID       string  `json:"member_id"`
	SubscriptionID *string `json:"subscription_id"`
	PTPackageID    *string `json:"pt_package_id"`
	Amount         float64 `json:"amount"`
	Method         string  `json:"method"`
	PaidAt         string  `json:"paid_at"` // RFC3339 or YYYY-MM-DD; default now
	Notes          string  `json:"notes"`
}

func (s *Server) handleCreatePayment(w http.ResponseWriter, r *http.Request) {
	var req paymentRequest
	if err := decode(r, &req); err != nil {
		s.respondErr(w, r, err)
		return
	}
	v := newValidator()
	in := store.PaymentInput{Amount: req.Amount, Method: req.Method, Notes: strings.TrimSpace(req.Notes), PaidAt: time.Now()}

	if id, err := uuid.Parse(req.MemberID); err != nil {
		v.add("member_id", "is required")
	} else {
		in.MemberID = id
	}
	if req.SubscriptionID != nil && *req.SubscriptionID != "" {
		if id, err := uuid.Parse(*req.SubscriptionID); err != nil {
			v.add("subscription_id", "is invalid")
		} else {
			in.SubscriptionID = &id
		}
	}
	if req.PTPackageID != nil && *req.PTPackageID != "" {
		if id, err := uuid.Parse(*req.PTPackageID); err != nil {
			v.add("pt_package_id", "is invalid")
		} else {
			in.PTPackageID = &id
		}
	}
	if in.SubscriptionID != nil && in.PTPackageID != nil {
		v.add("pt_package_id", "a payment can reference a subscription or a package, not both")
	}
	if in.Amount <= 0 {
		v.add("amount", "must be greater than 0")
	}
	if in.Method == "" {
		in.Method = "cash"
	}
	v.oneOf("method", in.Method, paymentMethods...)
	if req.PaidAt != "" {
		if t, err := parseFlexibleTime(req.PaidAt); err != nil {
			v.add("paid_at", "must be RFC3339 or YYYY-MM-DD")
		} else {
			in.PaidAt = t
		}
	}
	if err := v.err(); err != nil {
		s.respondErr(w, r, err)
		return
	}

	sess, _ := auth.FromContext(r.Context())
	var pm store.Payment
	err := s.tenantTx(r, func(tx pgx.Tx) (err error) {
		pm, err = store.CreatePayment(r.Context(), tx, sess.TenantID, in)
		return
	})
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusBadRequest, "member, subscription or package not found", nil)
			return
		}
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, pm)
}

func (s *Server) handleDeletePayment(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	err = s.tenantTx(r, func(tx pgx.Tx) error {
		return store.DeletePayment(r.Context(), tx, id)
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// parseFlexibleTime accepts RFC3339 or a bare date (interpreted at local noon
// to avoid day-boundary surprises).
func parseFlexibleTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02T15:04", s); err == nil {
		return t, nil
	}
	d, err := store.ParseDate(s)
	if err != nil {
		return time.Time{}, err
	}
	return d.Add(12 * time.Hour), nil
}
