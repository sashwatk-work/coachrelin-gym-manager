// Package api wires the HTTP router and handlers.
package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"

	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/auth"
	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/config"
	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/db"
	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/store"
)

// Server holds handler dependencies.
type Server struct {
	db     *db.DB
	tokens *auth.Tokens
	cfg    config.Config
	log    *slog.Logger
}

// New builds a Server.
func New(database *db.DB, tokens *auth.Tokens, cfg config.Config, log *slog.Logger) *Server {
	return &Server{db: database, tokens: tokens, cfg: cfg, log: log}
}

// Router returns the fully wired HTTP handler.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(s.logRequests)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", s.handleRegister)
			r.Post("/login", s.handleLogin)
			r.Post("/logout", s.handleLogout)
			r.With(auth.Middleware(s.tokens)).Get("/me", s.handleMe)
		})

		r.Group(func(r chi.Router) {
			r.Use(auth.Middleware(s.tokens))

			r.Get("/dashboard", s.handleDashboard)

			r.Route("/settings", func(r chi.Router) {
				r.Get("/tenant", s.handleGetTenant)
				r.With(s.roles(store.RoleOwner)).Patch("/tenant", s.handleUpdateTenant)
			})

			r.Route("/users", func(r chi.Router) {
				r.Use(s.roles(store.RoleOwner))
				r.Get("/", s.handleListUsers)
				r.Post("/", s.handleCreateUser)
				r.Put("/{id}", s.handleUpdateUser)
				r.Delete("/{id}", s.handleDeleteUser)
			})

			r.Route("/members", func(r chi.Router) {
				r.Get("/", s.handleListMembers)
				r.With(s.roles(store.RoleOwner, store.RoleReceptionist)).Post("/", s.handleCreateMember)
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", s.handleGetMember)
					r.With(s.roles(store.RoleOwner, store.RoleReceptionist)).Put("/", s.handleUpdateMember)
					r.With(s.roles(store.RoleOwner)).Delete("/", s.handleDeleteMember)
					r.With(s.roles(store.RoleOwner, store.RoleReceptionist)).Post("/subscriptions", s.handleCreateSubscription)
				})
			})

			r.Route("/subscriptions", func(r chi.Router) {
				r.With(s.roles(store.RoleOwner, store.RoleReceptionist)).Patch("/{id}", s.handleUpdateSubscription)
			})

			r.Route("/trainers", func(r chi.Router) {
				r.Get("/", s.handleListTrainers)
				r.With(s.roles(store.RoleOwner)).Post("/", s.handleCreateTrainer)
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", s.handleGetTrainer)
					r.With(s.roles(store.RoleOwner)).Put("/", s.handleUpdateTrainer)
					r.With(s.roles(store.RoleOwner)).Delete("/", s.handleDeleteTrainer)
				})
			})

			r.Route("/plans", func(r chi.Router) {
				r.With(s.roles(store.RoleOwner, store.RoleReceptionist)).Get("/", s.handleListPlans)
				r.With(s.roles(store.RoleOwner)).Post("/", s.handleCreatePlan)
				r.With(s.roles(store.RoleOwner)).Put("/{id}", s.handleUpdatePlan)
				r.With(s.roles(store.RoleOwner)).Delete("/{id}", s.handleDeletePlan)
			})

			r.Route("/payments", func(r chi.Router) {
				r.With(s.roles(store.RoleOwner, store.RoleReceptionist)).Get("/", s.handleListPayments)
				r.With(s.roles(store.RoleOwner, store.RoleReceptionist)).Post("/", s.handleCreatePayment)
				r.With(s.roles(store.RoleOwner)).Delete("/{id}", s.handleDeletePayment)
			})

			r.Route("/pt", func(r chi.Router) {
				r.Route("/plans", func(r chi.Router) {
					r.With(s.roles(store.RoleOwner, store.RoleReceptionist)).Get("/", s.handleListPTPlans)
					r.With(s.roles(store.RoleOwner)).Post("/", s.handleCreatePTPlan)
					r.With(s.roles(store.RoleOwner)).Put("/{id}", s.handleUpdatePTPlan)
					r.With(s.roles(store.RoleOwner)).Delete("/{id}", s.handleDeletePTPlan)
				})
				r.Route("/packages", func(r chi.Router) {
					r.Get("/", s.handleListPackages)
					r.With(s.roles(store.RoleOwner, store.RoleReceptionist)).Post("/", s.handleCreatePackage)
					r.Get("/{id}", s.handleGetPackage)
					r.With(s.roles(store.RoleOwner, store.RoleReceptionist)).Put("/{id}", s.handleUpdatePackage)
					r.With(s.roles(store.RoleOwner)).Delete("/{id}", s.handleDeletePackage)
				})
				r.Route("/sessions", func(r chi.Router) {
					r.Get("/", s.handleListSessions)
					r.Post("/", s.handleCreateSession)
					r.Put("/{id}", s.handleUpdateSession)
					r.Patch("/{id}", s.handleUpdateSessionStatus)
					r.With(s.roles(store.RoleOwner, store.RoleReceptionist)).Delete("/{id}", s.handleDeleteSession)
				})
			})
		})
	})

	return r
}

// tenantTx runs fn in a transaction scoped to the request's tenant.
func (s *Server) tenantTx(r *http.Request, fn func(tx pgx.Tx) error) error {
	sess, ok := auth.FromContext(r.Context())
	if !ok {
		return errStatus(http.StatusUnauthorized, "unauthenticated")
	}
	return s.db.WithTenantTx(r.Context(), sess.TenantID, fn)
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		s.log.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"ms", time.Since(start).Milliseconds(),
		)
	})
}
