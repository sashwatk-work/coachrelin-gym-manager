package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// subscriptionSelect returns the time-aware effective status: an "active"
// row whose end_date has passed is reported as "expired". "paid" is the sum
// of payments linked to this subscription plus any unlinked ("General")
// payments by the member that are allocated to it oldest-item-first (see
// migration 0002).
const subscriptionSelect = `
SELECT s.id, s.member_id, s.plan_id, p.name, s.start_date, s.end_date, s.price::float8,
  (lp.paid + CASE WHEN s.status = 'cancelled' THEN 0
                  ELSE allocated_credit(s.member_id, s.created_at, s.id, s.price - lp.paid) END)::float8 AS paid,
  CASE WHEN s.status = 'active' AND s.end_date < CURRENT_DATE THEN 'expired' ELSE s.status::text END AS status,
  s.created_at
FROM subscriptions s
JOIN membership_plans p ON p.id = s.plan_id
CROSS JOIN LATERAL (
  SELECT COALESCE((SELECT SUM(amount) FROM payments pay WHERE pay.subscription_id = s.id), 0) AS paid
) lp`

func scanSubscription(row pgx.Row, s *Subscription) error {
	return row.Scan(&s.ID, &s.MemberID, &s.PlanID, &s.PlanName, &s.StartDate, &s.EndDate, &s.Price, &s.Paid, &s.Status, &s.CreatedAt)
}

// ListMemberSubscriptions returns a member's subscriptions, newest first.
func ListMemberSubscriptions(ctx context.Context, q Querier, memberID uuid.UUID) ([]Subscription, error) {
	rows, err := q.Query(ctx, subscriptionSelect+` WHERE s.member_id = $1 ORDER BY s.end_date DESC, s.created_at DESC`, memberID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Subscription{}
	for rows.Next() {
		var s Subscription
		if err := scanSubscription(rows, &s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetSubscription fetches one subscription.
func GetSubscription(ctx context.Context, q Querier, id uuid.UUID) (Subscription, error) {
	var s Subscription
	err := scanSubscription(q.QueryRow(ctx, subscriptionSelect+` WHERE s.id = $1`, id), &s)
	if errors.Is(err, pgx.ErrNoRows) {
		return Subscription{}, ErrNotFound
	}
	return s, err
}

// SubscriptionInput is the writable subset of a subscription.
type SubscriptionInput struct {
	PlanID    uuid.UUID
	StartDate Date
	EndDate   Date
	Price     float64
}

// CreateSubscription inserts a subscription for a member. The plan must
// belong to the same tenant (enforced by the composite FK).
func CreateSubscription(ctx context.Context, q Querier, tenantID, memberID uuid.UUID, in SubscriptionInput) (Subscription, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx, `
		INSERT INTO subscriptions (tenant_id, member_id, plan_id, start_date, end_date, price)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		tenantID, memberID, in.PlanID, in.StartDate, in.EndDate, in.Price,
	).Scan(&id)
	if err != nil {
		if isFKViolation(err) {
			return Subscription{}, ErrNotFound
		}
		return Subscription{}, err
	}
	// Buying a membership reactivates an inactive member.
	if _, err := q.Exec(ctx, `UPDATE members SET status = 'active' WHERE id = $1 AND status = 'inactive'`, memberID); err != nil {
		return Subscription{}, err
	}
	return GetSubscription(ctx, q, id)
}

// SetSubscriptionStatus changes the stored status (active | cancelled).
func SetSubscriptionStatus(ctx context.Context, q Querier, id uuid.UUID, status string) (Subscription, error) {
	tag, err := q.Exec(ctx, `UPDATE subscriptions SET status = $2 WHERE id = $1`, id, status)
	if err != nil {
		return Subscription{}, err
	}
	if tag.RowsAffected() == 0 {
		return Subscription{}, ErrNotFound
	}
	return GetSubscription(ctx, q, id)
}
