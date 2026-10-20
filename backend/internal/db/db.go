// Package db owns the connection pool, migrations and the tenant-scoped
// transaction helper that every tenant query must go through.
package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/sashwatk-work/coachrelin-gym-manager/backend/migrations"
)

// DB wraps a pgx pool.
type DB struct {
	Pool *pgxpool.Pool
}

// Connect opens a pool and verifies connectivity.
func Connect(ctx context.Context, url string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MaxConnLifetime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &DB{Pool: pool}, nil
}

// Close releases the pool.
func (d *DB) Close() { d.Pool.Close() }

// Migrate applies all pending goose migrations from the embedded FS.
func (d *DB) Migrate(ctx context.Context) error {
	sqlDB := stdlib.OpenDBFromPool(d.Pool)
	defer sqlDB.Close()

	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	if err := goose.UpContext(ctx, sqlDB, "."); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

// ErrUnsafeRole is returned when the connected role would bypass RLS.
var ErrUnsafeRole = errors.New("database role is a superuser or has BYPASSRLS; row-level security would not be enforced")

// ErrTenantNotFound is returned by WithTenantTx when the tenant no longer exists.
var ErrTenantNotFound = errors.New("tenant not found")

// CheckRLSSafety fails if the connected role can bypass row-level security.
// Superusers and BYPASSRLS roles ignore RLS even when it is forced, which
// would silently break tenant isolation.
func (d *DB) CheckRLSSafety(ctx context.Context) error {
	var unsafe bool
	err := d.Pool.QueryRow(ctx,
		`SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`,
	).Scan(&unsafe)
	if err != nil {
		return fmt.Errorf("check role: %w", err)
	}
	if unsafe {
		return ErrUnsafeRole
	}
	return nil
}

// WithTenantTx runs fn inside a transaction scoped to tenantID. It sets the
// app.tenant_id session variable with SET LOCAL so that the RLS policies on
// every domain table only expose that tenant's rows. The variable is reset
// automatically when the transaction ends.
func (d *DB) WithTenantTx(ctx context.Context, tenantID uuid.UUID, fn func(tx pgx.Tx) error) error {
	if tenantID == uuid.Nil {
		return errors.New("WithTenantTx: tenant id is required")
	}
	return d.withTx(ctx, func(tx pgx.Tx) error {
		// SET LOCAL does not support bind parameters, so use set_config
		// (is_local = true scopes it to this transaction).
		if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenantID.String()); err != nil {
			return fmt.Errorf("set tenant: %w", err)
		}
		// Use the tenant's timezone so CURRENT_DATE / "today" match the gym's
		// local calendar. Also confirms the tenant still exists.
		var tz string
		if err := tx.QueryRow(ctx, `SELECT timezone FROM tenants WHERE id = $1`, tenantID).Scan(&tz); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrTenantNotFound
			}
			return fmt.Errorf("load tenant: %w", err)
		}
		if _, err := tx.Exec(ctx, `SELECT set_config('TimeZone', $1, true)`, tz); err != nil {
			return fmt.Errorf("set timezone: %w", err)
		}
		return fn(tx)
	})
}

// WithTx runs fn inside a plain transaction with no tenant set. Only platform
// tables (tenants, users) are readable in this mode; RLS hides every domain row.
func (d *DB) WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return d.withTx(ctx, fn)
}

func (d *DB) withTx(ctx context.Context, fn func(tx pgx.Tx) error) (err error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			return
		}
		err = tx.Commit(ctx)
	}()
	return fn(tx)
}
