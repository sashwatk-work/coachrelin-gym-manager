package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const planSelect = `
SELECT p.id, p.name, p.duration_days, p.price::float8, p.description, p.active, p.created_at, p.updated_at,
  (SELECT COUNT(*) FROM subscriptions s WHERE s.plan_id = p.id AND s.status = 'active' AND s.end_date >= CURRENT_DATE) AS active_subscriptions
FROM membership_plans p`

func scanPlan(row pgx.Row, p *MembershipPlan) error {
	return row.Scan(&p.ID, &p.Name, &p.DurationDays, &p.Price, &p.Description, &p.Active, &p.CreatedAt, &p.UpdatedAt,
		&p.ActiveSubscriptions)
}

// ListPlans returns all plans. If activeOnly, inactive plans are hidden.
func ListPlans(ctx context.Context, q Querier, activeOnly bool) ([]MembershipPlan, error) {
	rows, err := q.Query(ctx, planSelect+`
		WHERE (NOT $1 OR p.active)
		ORDER BY p.active DESC, p.duration_days, p.price, p.name`, activeOnly)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MembershipPlan{}
	for rows.Next() {
		var p MembershipPlan
		if err := scanPlan(rows, &p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetPlan fetches one plan.
func GetPlan(ctx context.Context, q Querier, id uuid.UUID) (MembershipPlan, error) {
	var p MembershipPlan
	err := scanPlan(q.QueryRow(ctx, planSelect+` WHERE p.id = $1`, id), &p)
	if errors.Is(err, pgx.ErrNoRows) {
		return MembershipPlan{}, ErrNotFound
	}
	return p, err
}

// PlanInput is the writable subset of a plan.
type PlanInput struct {
	Name         string
	DurationDays int
	Price        float64
	Description  string
	Active       bool
}

// CreatePlan inserts a plan.
func CreatePlan(ctx context.Context, q Querier, tenantID uuid.UUID, in PlanInput) (MembershipPlan, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx, `
		INSERT INTO membership_plans (tenant_id, name, duration_days, price, description, active)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		tenantID, in.Name, in.DurationDays, in.Price, in.Description, in.Active,
	).Scan(&id)
	if err != nil {
		return MembershipPlan{}, err
	}
	return GetPlan(ctx, q, id)
}

// UpdatePlan replaces a plan's editable fields.
func UpdatePlan(ctx context.Context, q Querier, id uuid.UUID, in PlanInput) (MembershipPlan, error) {
	tag, err := q.Exec(ctx, `
		UPDATE membership_plans SET name = $2, duration_days = $3, price = $4, description = $5, active = $6
		WHERE id = $1`,
		id, in.Name, in.DurationDays, in.Price, in.Description, in.Active)
	if err != nil {
		return MembershipPlan{}, err
	}
	if tag.RowsAffected() == 0 {
		return MembershipPlan{}, ErrNotFound
	}
	return GetPlan(ctx, q, id)
}

// DeletePlan removes a plan. Returns ErrConflict if subscriptions reference it.
func DeletePlan(ctx context.Context, q Querier, id uuid.UUID) error {
	tag, err := q.Exec(ctx, `DELETE FROM membership_plans WHERE id = $1`, id)
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
