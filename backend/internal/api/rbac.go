package api

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/auth"
	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/store"
)

func requireRoles(r *http.Request, allowed ...string) error {
	sess, ok := auth.FromContext(r.Context())
	if !ok {
		return errStatus(http.StatusUnauthorized, "unauthenticated")
	}
	if sess.HasAny(allowed...) {
		return nil
	}
	return errStatus(http.StatusForbidden, "you do not have permission to do this")
}

func (s *Server) roles(allowed ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := requireRoles(r, allowed...); err != nil {
				s.respondErr(w, r, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func sessionOf(r *http.Request) auth.Session {
	sess, _ := auth.FromContext(r.Context())
	return sess
}

func trainerIDOfUser(u store.User) uuid.UUID {
	if u.TrainerID != nil {
		return *u.TrainerID
	}
	return uuid.Nil
}

// trainerOnly is true when the login is a trainer without owner or receptionist,
// so member/PT lists stay locked to that trainer.
func trainerOnly(sess auth.Session) bool {
	return sess.Has(store.RoleTrainer) && !sess.Has(store.RoleOwner) && !sess.Has(store.RoleReceptionist)
}

// scopedTrainerID returns the trainer id that list/get queries must be locked
// to. Broader roles (owner, receptionist) are not filtered.
func scopedTrainerID(r *http.Request) *uuid.UUID {
	sess := sessionOf(r)
	if !trainerOnly(sess) {
		return nil
	}
	id := sess.TrainerID
	return &id
}

func requireLinkedTrainer(r *http.Request) error {
	sess := sessionOf(r)
	if sess.Has(store.RoleTrainer) && sess.TrainerID == uuid.Nil {
		return errStatus(http.StatusForbidden, "this trainer account is not linked to a trainer profile")
	}
	return nil
}
