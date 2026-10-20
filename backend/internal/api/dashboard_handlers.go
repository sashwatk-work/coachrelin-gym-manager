package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/auth"
	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/store"
)

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if err := requireLinkedTrainer(r); err != nil {
		s.respondErr(w, r, err)
		return
	}
	sess := sessionOf(r)
	var d store.Dashboard
	err := s.tenantTx(r, func(tx pgx.Tx) (err error) {
		if trainerOnly(sess) {
			d, err = store.GetTrainerDashboard(r.Context(), tx, sess.TrainerID)
			return
		}
		d, err = store.GetDashboard(r.Context(), tx)
		return
	})
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) handleGetTenant(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	t, err := store.GetTenant(r.Context(), s.db.Pool, sess.TenantID)
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

type tenantPatch struct {
	Name       *string  `json:"name"`
	Timezone   *string  `json:"timezone"`
	Currency   *string  `json:"currency"`
	Gstin      *string  `json:"gstin"`
	GstPercent *float64 `json:"gst_percent"`
}

func (s *Server) handleUpdateTenant(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	var req tenantPatch
	if err := decode(r, &req); err != nil {
		s.respondErr(w, r, err)
		return
	}
	cur, err := store.GetTenant(r.Context(), s.db.Pool, sess.TenantID)
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	v := newValidator()
	u := store.TenantUpdate{Name: cur.Name, Timezone: cur.Timezone, Currency: cur.Currency, Gstin: cur.Gstin, GstPercent: cur.GstPercent}
	if req.Name != nil {
		u.Name = v.require("name", *req.Name)
	}
	if req.Timezone != nil {
		tz := strings.TrimSpace(*req.Timezone)
		if _, err := time.LoadLocation(tz); err != nil || tz == "" {
			v.add("timezone", "must be a valid IANA timezone")
		} else {
			u.Timezone = tz
		}
	}
	if req.Currency != nil {
		c := strings.ToUpper(strings.TrimSpace(*req.Currency))
		if len(c) != 3 {
			v.add("currency", "must be a 3-letter ISO code")
		} else {
			u.Currency = c
		}
	}
	if req.Gstin != nil {
		g, err := store.NormalizeGSTIN(*req.Gstin)
		if err != nil {
			v.add("gstin", err.Error())
		} else if g == "" {
			u.Gstin = nil
		} else {
			u.Gstin = &g
		}
	}
	if req.GstPercent != nil {
		p, err := store.NormalizeGSTPercent(*req.GstPercent)
		if err != nil {
			v.add("gst_percent", err.Error())
		} else {
			u.GstPercent = p
		}
	}
	if err := v.err(); err != nil {
		s.respondErr(w, r, err)
		return
	}
	t, err := store.UpdateTenant(r.Context(), s.db.Pool, sess.TenantID, u)
	if err != nil {
		s.respondErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}
