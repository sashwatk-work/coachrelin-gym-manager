package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const ptPlanSelect = `
SELECT p.id, p.name, p.sessions, p.validity_days, p.price::float8, p.description, p.active, p.created_at, p.updated_at,
  (SELECT COUNT(*) FROM pt_packages pk WHERE pk.pt_plan_id = p.id AND pk.status = 'active'
     AND (pk.expires_at IS NULL OR pk.expires_at >= CURRENT_DATE)) AS active_packages
FROM pt_plans p`

func scanPTPlan(row pgx.Row, p *PTPlan) error {
	return row.Scan(&p.ID, &p.Name, &p.Sessions, &p.ValidityDays, &p.Price, &p.Description, &p.Active,
		&p.CreatedAt, &p.UpdatedAt, &p.ActivePackages)
}

// ListPTPlans returns all PT plans. If activeOnly, inactive plans are hidden.
func ListPTPlans(ctx context.Context, q Querier, activeOnly bool) ([]PTPlan, error) {
	rows, err := q.Query(ctx, ptPlanSelect+`
		WHERE (NOT $1 OR p.active)
		ORDER BY p.active DESC, p.sessions, p.price, p.name`, activeOnly)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PTPlan{}
	for rows.Next() {
		var p PTPlan
		if err := scanPTPlan(rows, &p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetPTPlan fetches one PT plan.
func GetPTPlan(ctx context.Context, q Querier, id uuid.UUID) (PTPlan, error) {
	var p PTPlan
	err := scanPTPlan(q.QueryRow(ctx, ptPlanSelect+` WHERE p.id = $1`, id), &p)
	if errors.Is(err, pgx.ErrNoRows) {
		return PTPlan{}, ErrNotFound
	}
	return p, err
}

// PTPlanInput is the writable subset of a PT plan.
type PTPlanInput struct {
	Name         string
	Sessions     int
	ValidityDays *int
	Price        float64
	Description  string
	Active       bool
}

// CreatePTPlan inserts a PT plan.
func CreatePTPlan(ctx context.Context, q Querier, tenantID uuid.UUID, in PTPlanInput) (PTPlan, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx, `
		INSERT INTO pt_plans (tenant_id, name, sessions, validity_days, price, description, active)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		tenantID, in.Name, in.Sessions, in.ValidityDays, in.Price, in.Description, in.Active,
	).Scan(&id)
	if err != nil {
		return PTPlan{}, err
	}
	return GetPTPlan(ctx, q, id)
}

// UpdatePTPlan replaces a PT plan's editable fields. Packages already sold
// from the plan are not affected.
func UpdatePTPlan(ctx context.Context, q Querier, id uuid.UUID, in PTPlanInput) (PTPlan, error) {
	tag, err := q.Exec(ctx, `
		UPDATE pt_plans SET name = $2, sessions = $3, validity_days = $4, price = $5, description = $6, active = $7
		WHERE id = $1`,
		id, in.Name, in.Sessions, in.ValidityDays, in.Price, in.Description, in.Active)
	if err != nil {
		return PTPlan{}, err
	}
	if tag.RowsAffected() == 0 {
		return PTPlan{}, ErrNotFound
	}
	return GetPTPlan(ctx, q, id)
}

// DeletePTPlan removes a PT plan. Returns ErrConflict if packages reference it.
func DeletePTPlan(ctx context.Context, q Querier, id uuid.UUID) error {
	tag, err := q.Exec(ctx, `DELETE FROM pt_plans WHERE id = $1`, id)
	if err != nil {
		if isRestrictViolation(err) {
			return ErrConflict
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
