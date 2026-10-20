package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const paymentSelect = `
SELECT pay.id, pay.member_id, m.full_name, pay.subscription_id, pay.pt_package_id, pay.amount::float8,
  pay.method::text, pay.paid_at, pay.notes, pay.created_at,
  COALESCE(
    (SELECT 'Membership: ' || p.name FROM subscriptions s JOIN membership_plans p ON p.id = s.plan_id WHERE s.id = pay.subscription_id),
    (SELECT 'PT: ' || pk.name FROM pt_packages pk WHERE pk.id = pay.pt_package_id),
    'General'
  ) AS reference_label
FROM payments pay
JOIN members m ON m.id = pay.member_id`

func scanPayment(row pgx.Row, p *Payment) error {
	return row.Scan(&p.ID, &p.MemberID, &p.MemberName, &p.SubscriptionID, &p.PTPackageID, &p.Amount,
		&p.Method, &p.PaidAt, &p.Notes, &p.CreatedAt, &p.ReferenceLabel)
}

// PaymentListParams filters the payments ledger.
type PaymentListParams struct {
	ListParams
	MemberID *uuid.UUID
	Method   string
	From     *Date
	To       *Date
}

// PaymentPage is a page of payments plus the sum of the filtered set.
type PaymentPage struct {
	Page[Payment]
	TotalAmount float64 `json:"total_amount"`
}

// ListPayments returns a filtered page of payments, newest first.
func ListPayments(ctx context.Context, q Querier, p PaymentListParams) (PaymentPage, error) {
	p.Normalize()
	out := PaymentPage{Page: Page[Payment]{Items: []Payment{}, Page: p.Page, PageSize: p.PageSize}}

	where := `
		WHERE ($1 = '' OR m.full_name ILIKE $2 OR pay.notes ILIKE $2)
		  AND ($3::uuid IS NULL OR pay.member_id = $3)
		  AND ($4 = '' OR pay.method::text = $4)
		  AND ($5::date IS NULL OR pay.paid_at::date >= $5)
		  AND ($6::date IS NULL OR pay.paid_at::date <= $6)`
	args := []any{p.Q, like(p.Q), p.MemberID, p.Method, p.From, p.To}

	rows, err := q.Query(ctx, paymentSelect+where+`
		ORDER BY pay.paid_at DESC, pay.created_at DESC
		LIMIT $7 OFFSET $8`, append(args, p.PageSize, p.Offset())...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var pm Payment
		if err := scanPayment(rows, &pm); err != nil {
			return out, err
		}
		out.Items = append(out.Items, pm)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}

	err = q.QueryRow(ctx, `SELECT COUNT(*), COALESCE(SUM(pay.amount), 0)::float8 FROM payments pay JOIN members m ON m.id = pay.member_id`+where, args...).
		Scan(&out.Total, &out.TotalAmount)
	return out, err
}

// ListMemberPayments returns all payments for a member.
func ListMemberPayments(ctx context.Context, q Querier, memberID uuid.UUID) ([]Payment, error) {
	rows, err := q.Query(ctx, paymentSelect+` WHERE pay.member_id = $1 ORDER BY pay.paid_at DESC, pay.created_at DESC`, memberID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Payment{}
	for rows.Next() {
		var pm Payment
		if err := scanPayment(rows, &pm); err != nil {
			return nil, err
		}
		out = append(out, pm)
	}
	return out, rows.Err()
}

// GetPayment fetches one payment.
func GetPayment(ctx context.Context, q Querier, id uuid.UUID) (Payment, error) {
	var pm Payment
	err := scanPayment(q.QueryRow(ctx, paymentSelect+` WHERE pay.id = $1`, id), &pm)
	if errors.Is(err, pgx.ErrNoRows) {
		return Payment{}, ErrNotFound
	}
	return pm, err
}

// PaymentInput is the writable subset of a payment.
type PaymentInput struct {
	MemberID       uuid.UUID
	SubscriptionID *uuid.UUID
	PTPackageID    *uuid.UUID
	Amount         float64
	Method         string
	PaidAt         time.Time
	Notes          string
}

// CreatePayment records a payment. Linked subscription/package must belong to
// the same member; this is validated here because the FKs only check tenant.
func CreatePayment(ctx context.Context, q Querier, tenantID uuid.UUID, in PaymentInput) (Payment, error) {
	if in.SubscriptionID != nil {
		var ok bool
		if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM subscriptions WHERE id = $1 AND member_id = $2)`,
			*in.SubscriptionID, in.MemberID).Scan(&ok); err != nil {
			return Payment{}, err
		}
		if !ok {
			return Payment{}, ErrNotFound
		}
	}
	if in.PTPackageID != nil {
		var ok bool
		if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pt_packages WHERE id = $1 AND member_id = $2)`,
			*in.PTPackageID, in.MemberID).Scan(&ok); err != nil {
			return Payment{}, err
		}
		if !ok {
			return Payment{}, ErrNotFound
		}
	}
	var id uuid.UUID
	err := q.QueryRow(ctx, `
		INSERT INTO payments (tenant_id, member_id, subscription_id, pt_package_id, amount, method, paid_at, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		tenantID, in.MemberID, in.SubscriptionID, in.PTPackageID, in.Amount, in.Method, in.PaidAt, in.Notes,
	).Scan(&id)
	if err != nil {
		if isFKViolation(err) {
			return Payment{}, ErrNotFound
		}
		return Payment{}, err
	}
	return GetPayment(ctx, q, id)
}

// DeletePayment removes a payment record.
func DeletePayment(ctx context.Context, q Querier, id uuid.UUID) error {
	tag, err := q.Exec(ctx, `DELETE FROM payments WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
