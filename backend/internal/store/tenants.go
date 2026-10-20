package store

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// Indian GSTIN: 2-digit state, PAN, entity, Z, checksum.
var gstinRe = regexp.MustCompile(`^[0-9]{2}[A-Z]{5}[0-9]{4}[A-Z][1-9A-Z]Z[0-9A-Z]$`)

const tenantSelect = `id, name, slug, timezone, currency, gstin, gst_percent, created_at`

func scanTenant(row pgx.Row, t *Tenant) error {
	return row.Scan(&t.ID, &t.Name, &t.Slug, &t.Timezone, &t.Currency, &t.Gstin, &t.GstPercent, &t.CreatedAt)
}

// Slugify turns "Coach Relin Fitness" into "coach-relin-fitness".
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "gym"
	}
	return s
}

// NormalizeGSTIN uppercases and strips spaces. Empty is valid (unregistered).
func NormalizeGSTIN(s string) (string, error) {
	s = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(s), " ", ""))
	if s == "" {
		return "", nil
	}
	if !gstinRe.MatchString(s) {
		return "", fmt.Errorf("must be a 15-character GSTIN")
	}
	return s, nil
}

// NormalizeGSTPercent accepts 0–100, rounded to two decimals.
func NormalizeGSTPercent(n float64) (float64, error) {
	if math.IsNaN(n) || n < 0 || n > 100 {
		return 0, fmt.Errorf("must be between 0 and 100")
	}
	return math.Round(n*100) / 100, nil
}

// CreateTenant inserts a tenant. If the slug collides, a numeric suffix is added.
func CreateTenant(ctx context.Context, q Querier, name, timezone, currency string) (Tenant, error) {
	base := Slugify(name)
	for i := 0; i < 20; i++ {
		slug := base
		if i > 0 {
			slug = fmt.Sprintf("%s-%d", base, i+1)
		}
		var t Tenant
		err := scanTenant(q.QueryRow(ctx, `
			INSERT INTO tenants (name, slug, timezone, currency)
			VALUES ($1, $2, $3, $4)
			RETURNING `+tenantSelect,
			name, slug, timezone, currency,
		), &t)
		if err == nil {
			return t, nil
		}
		if !isUniqueViolation(err) {
			return Tenant{}, err
		}
	}
	return Tenant{}, ErrConflict
}

// GetTenant fetches a tenant by id.
func GetTenant(ctx context.Context, q Querier, id uuid.UUID) (Tenant, error) {
	var t Tenant
	err := scanTenant(q.QueryRow(ctx, `SELECT `+tenantSelect+` FROM tenants WHERE id = $1`, id), &t)
	if errors.Is(err, pgx.ErrNoRows) {
		return Tenant{}, ErrNotFound
	}
	return t, err
}

// GetTenantBySlug fetches a tenant by slug.
func GetTenantBySlug(ctx context.Context, q Querier, slug string) (Tenant, error) {
	var t Tenant
	err := scanTenant(q.QueryRow(ctx, `SELECT `+tenantSelect+` FROM tenants WHERE slug = $1`, slug), &t)
	if errors.Is(err, pgx.ErrNoRows) {
		return Tenant{}, ErrNotFound
	}
	return t, err
}

// TenantUpdate holds editable tenant settings.
type TenantUpdate struct {
	Name       string
	Timezone   string
	Currency   string
	Gstin      *string
	GstPercent float64
}

// UpdateTenant changes tenant settings.
func UpdateTenant(ctx context.Context, q Querier, id uuid.UUID, u TenantUpdate) (Tenant, error) {
	var t Tenant
	err := scanTenant(q.QueryRow(ctx, `
		UPDATE tenants SET name = $2, timezone = $3, currency = $4, gstin = $5, gst_percent = $6
		WHERE id = $1
		RETURNING `+tenantSelect,
		id, u.Name, u.Timezone, u.Currency, u.Gstin, u.GstPercent,
	), &t)
	if errors.Is(err, pgx.ErrNoRows) {
		return Tenant{}, ErrNotFound
	}
	return t, err
}

// DeleteTenant removes a tenant and (via cascade) all of its data.
func DeleteTenant(ctx context.Context, q Querier, id uuid.UUID) error {
	_, err := q.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, id)
	return err
}
