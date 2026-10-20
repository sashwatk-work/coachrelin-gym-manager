package db

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// TestRowLevelSecurity proves tenant isolation at the database layer, below
// any application code: rows written under one tenant are invisible under
// another, invisible with no tenant set at all, and a transaction scoped to
// tenant A cannot smuggle rows into tenant B.
//
// Skipped unless TEST_DATABASE_URL is set.
func TestRowLevelSecurity(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping RLS integration test")
	}
	ctx := context.Background()
	d, err := Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	// Registered first so it runs last, after the tenant cleanup below.
	t.Cleanup(d.Close)

	if err := d.CheckRLSSafety(ctx); err != nil {
		t.Fatalf("the test role must not be a superuser or BYPASSRLS, otherwise RLS is silently off: %v", err)
	}
	if err := d.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Every domain table must have RLS enabled AND forced (forced so even the
	// table owner is subject to the policy).
	rows, err := d.Pool.Query(ctx, `
		SELECT c.relname, c.relrowsecurity, c.relforcerowsecurity
		FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relkind = 'r'
		  AND c.relname IN ('trainers','membership_plans','members','subscriptions','pt_plans','pt_packages','pt_sessions','payments')`)
	if err != nil {
		t.Fatalf("query pg_class: %v", err)
	}
	seen := 0
	for rows.Next() {
		var name string
		var enabled, forced bool
		if err := rows.Scan(&name, &enabled, &forced); err != nil {
			t.Fatal(err)
		}
		seen++
		if !enabled || !forced {
			t.Errorf("table %s: rowsecurity=%v forcerowsecurity=%v; want both true", name, enabled, forced)
		}
	}
	rows.Close()
	if seen != 8 {
		t.Fatalf("found %d domain tables with RLS metadata, want 8", seen)
	}

	// Two throwaway tenants.
	var tenantA, tenantB uuid.UUID
	suffix := uuid.NewString()[:8]
	err = d.WithTx(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx,
			`INSERT INTO tenants (name, slug, timezone, currency) VALUES ($1, $2, 'UTC', 'USD') RETURNING id`,
			"RLS Test A "+suffix, "rls-test-a-"+suffix).Scan(&tenantA); err != nil {
			return err
		}
		return tx.QueryRow(ctx,
			`INSERT INTO tenants (name, slug, timezone, currency) VALUES ($1, $2, 'UTC', 'USD') RETURNING id`,
			"RLS Test B "+suffix, "rls-test-b-"+suffix).Scan(&tenantB)
	})
	if err != nil {
		t.Fatalf("create tenants: %v", err)
	}
	t.Cleanup(func() {
		err := d.WithTx(ctx, func(tx pgx.Tx) error {
			// Also sweep tenants left behind by earlier interrupted runs.
			_, err := tx.Exec(ctx, `DELETE FROM tenants WHERE id = $1 OR id = $2 OR slug LIKE 'rls-test-%'`, tenantA, tenantB)
			return err
		})
		if err != nil {
			t.Errorf("cleanup test tenants: %v", err)
		}
	})

	countMembers := func(t *testing.T, tx pgx.Tx) int {
		t.Helper()
		var n int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM members`).Scan(&n); err != nil {
			t.Fatalf("count members: %v", err)
		}
		return n
	}

	// Insert a member as tenant A.
	var memberA uuid.UUID
	err = d.WithTenantTx(ctx, tenantA, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`INSERT INTO members (tenant_id, full_name, joined_at) VALUES ($1, 'Alice', CURRENT_DATE) RETURNING id`,
			tenantA).Scan(&memberA)
	})
	if err != nil {
		t.Fatalf("insert member as A: %v", err)
	}

	t.Run("visible to owning tenant", func(t *testing.T) {
		err := d.WithTenantTx(ctx, tenantA, func(tx pgx.Tx) error {
			if n := countMembers(t, tx); n != 1 {
				t.Errorf("tenant A sees %d members, want 1", n)
			}
			var name string
			if err := tx.QueryRow(ctx, `SELECT full_name FROM members WHERE id = $1`, memberA).Scan(&name); err != nil {
				return err
			}
			if name != "Alice" {
				t.Errorf("name = %q", name)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invisible to other tenant", func(t *testing.T) {
		err := d.WithTenantTx(ctx, tenantB, func(tx pgx.Tx) error {
			if n := countMembers(t, tx); n != 0 {
				t.Errorf("tenant B sees %d members, want 0", n)
			}
			var name string
			err := tx.QueryRow(ctx, `SELECT full_name FROM members WHERE id = $1`, memberA).Scan(&name)
			if !errors.Is(err, pgx.ErrNoRows) {
				t.Errorf("lookup of A's member by id under B: err=%v name=%q; want no rows", err, name)
			}
			// Updates and deletes silently affect zero rows.
			tag, err := tx.Exec(ctx, `UPDATE members SET full_name = 'Hijacked' WHERE id = $1`, memberA)
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 0 {
				t.Errorf("B updated %d of A's rows", tag.RowsAffected())
			}
			tag, err = tx.Exec(ctx, `DELETE FROM members WHERE id = $1`, memberA)
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 0 {
				t.Errorf("B deleted %d of A's rows", tag.RowsAffected())
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invisible with no tenant set", func(t *testing.T) {
		err := d.WithTx(ctx, func(tx pgx.Tx) error {
			if n := countMembers(t, tx); n != 0 {
				t.Errorf("with no app.tenant_id, %d members are visible; want 0", n)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("cannot write rows for another tenant", func(t *testing.T) {
		err := d.WithTenantTx(ctx, tenantA, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx,
				`INSERT INTO members (tenant_id, full_name, joined_at) VALUES ($1, 'Mallory', CURRENT_DATE)`, tenantB)
			return err
		})
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Fatalf("insert into B under A's tx: err = %v; want RLS violation 42501", err)
		}

		// Nor re-home an existing row.
		err = d.WithTenantTx(ctx, tenantA, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `UPDATE members SET tenant_id = $1 WHERE id = $2`, tenantB, memberA)
			return err
		})
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Fatalf("move A's row to B: err = %v; want RLS violation 42501", err)
		}
	})

	t.Run("tenant setting is transaction-local", func(t *testing.T) {
		// After a tenant-scoped transaction ends, the pooled connection must
		// not retain app.tenant_id, otherwise a later untenanted query could
		// leak rows. Exhaust the pool briefly so we're likely to reuse conns.
		for i := 0; i < 20; i++ {
			err := d.WithTx(ctx, func(tx pgx.Tx) error {
				var setting string
				if err := tx.QueryRow(ctx, `SELECT current_setting('app.tenant_id', true)`).Scan(&setting); err != nil {
					return err
				}
				if setting != "" {
					t.Errorf("app.tenant_id leaked across transactions: %q", setting)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		}
	})

	t.Run("unknown tenant is rejected", func(t *testing.T) {
		err := d.WithTenantTx(ctx, uuid.New(), func(pgx.Tx) error { return nil })
		if !errors.Is(err, ErrTenantNotFound) {
			t.Fatalf("err = %v, want ErrTenantNotFound", err)
		}
		if err := d.WithTenantTx(ctx, uuid.Nil, func(pgx.Tx) error { return nil }); err == nil {
			t.Fatal("nil tenant id must be rejected")
		}
	})

	t.Run("tenant deletion cascades", func(t *testing.T) {
		err := d.WithTx(ctx, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenantA)
			return err
		})
		if err != nil {
			t.Fatalf("delete tenant A: %v", err)
		}
		// A's id can no longer be used to scope a transaction, and its rows
		// are gone (the FK on members.tenant_id is ON DELETE CASCADE, which
		// is what makes this delete succeed in the first place).
		err = d.WithTenantTx(ctx, tenantA, func(pgx.Tx) error { return nil })
		if !errors.Is(err, ErrTenantNotFound) {
			t.Fatalf("after delete, WithTenantTx(A) err = %v, want ErrTenantNotFound", err)
		}
	})
}
