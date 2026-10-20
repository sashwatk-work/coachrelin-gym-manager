// Command api serves the gym manager HTTP API.
//
//	api            run the server
//	api -migrate   apply migrations and exit
//	api -seed      seed optional demo members for Coach Relin Fitness and exit
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/api"
	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/auth"
	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/config"
	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/db"
	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/seed"
)

func main() {
	migrateOnly := flag.Bool("migrate", false, "run migrations and exit")
	seedOnly := flag.Bool("seed", false, "seed demo data and exit")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := run(context.Background(), log, *migrateOnly, *seedOnly); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, log *slog.Logger, migrateOnly, seedOnly bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	database, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer database.Close()

	if err := database.CheckRLSSafety(ctx); err != nil {
		if errors.Is(err, db.ErrUnsafeRole) && cfg.AllowSuperuser {
			log.Warn("RLS NOT ENFORCED: connected as superuser/BYPASSRLS role (ALLOW_SUPERUSER=true)")
		} else {
			return fmt.Errorf("%w (set ALLOW_SUPERUSER=true to override for local experiments)", err)
		}
	}

	if migrateOnly || cfg.AutoMigrate {
		log.Info("running migrations")
		if err := database.Migrate(ctx); err != nil {
			return err
		}
		log.Info("migrations complete")
	}

	res, err := seed.EnsureGym(ctx, database)
	if err != nil {
		return fmt.Errorf("ensure gym: %w", err)
	}
	if res.Created {
		log.Info("created Coach Relin Fitness", "owner", res.OwnerEmail)
	}

	if seedOnly || cfg.SeedOnStart {
		log.Info("seeding demo members")
		res, err := seed.Run(ctx, database)
		if err != nil {
			return err
		}
		log.Info("seed complete", "tenant", res.Tenant.Name, "owner", res.OwnerEmail, "created", res.Created)
	}
	if migrateOnly || seedOnly {
		return nil
	}

	tokens := auth.NewTokens(cfg.JWTSecret, cfg.SessionTTL)
	srv := api.New(database, tokens, cfg, log)

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Info("api listening", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	}
}
