package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// memberSelect joins each member with its most relevant subscription and the
// member's outstanding dues. Relies on the tx TimeZone being set to the tenant
// timezone so CURRENT_DATE is the gym's local date.
//
// Dues = what the member was billed for (non-cancelled subscriptions and PT
// packages) minus payments linked to those items, minus any unlinked
// ("General") payments, floored at zero. Unlinked payments therefore always
// reduce dues even when the owner did not pick what they were for.
const memberSelect = `
WITH base AS (
  SELECT m.id, m.full_name, m.email, m.phone, m.date_of_birth, m.gender, m.joined_at,
         m.status::text AS status, m.emergency_contact, m.notes, m.created_at, m.updated_at,
         cs.plan_name, cs.end_date,
         CASE
           WHEN cs.end_date IS NULL THEN 'none'
           WHEN cs.end_date < CURRENT_DATE THEN 'expired'
           WHEN cs.end_date <= CURRENT_DATE + 7 THEN 'expiring'
           ELSE 'active'
         END AS membership_status,
         COALESCE(d.dues, 0)::float8 AS dues
  FROM members m
  LEFT JOIN LATERAL (
    SELECT p.name AS plan_name, s.end_date
    FROM subscriptions s
    JOIN membership_plans p ON p.id = s.plan_id
    WHERE s.member_id = m.id AND s.status <> 'cancelled'
    ORDER BY (s.end_date >= CURRENT_DATE) DESC, s.end_date DESC
    LIMIT 1
  ) cs ON true
  LEFT JOIN LATERAL (
    SELECT GREATEST(SUM(GREATEST(x.price - x.paid, 0)) - member_unlinked_credit(m.id), 0) AS dues FROM (
      SELECT s.price, COALESCE((SELECT SUM(amount) FROM payments pay WHERE pay.subscription_id = s.id), 0) AS paid
      FROM subscriptions s WHERE s.member_id = m.id AND s.status <> 'cancelled'
      UNION ALL
      SELECT pk.price, COALESCE((SELECT SUM(amount) FROM payments pay WHERE pay.pt_package_id = pk.id), 0)
      FROM pt_packages pk WHERE pk.member_id = m.id AND pk.status <> 'cancelled'
    ) x
  ) d ON true
)
`

const memberCols = `id, full_name, email, phone, date_of_birth, gender, joined_at, status,
  emergency_contact, notes, created_at, updated_at, plan_name, end_date, membership_status, dues`

func scanMember(row pgx.Row, m *Member) error {
	var dob, end *Date
	err := row.Scan(&m.ID, &m.FullName, &m.Email, &m.Phone, &dob, &m.Gender, &m.JoinedAt, &m.Status,
		&m.EmergencyContact, &m.Notes, &m.CreatedAt, &m.UpdatedAt,
		&m.CurrentPlanName, &end, &m.MembershipStatus, &m.OutstandingDues)
	if err != nil {
		return err
	}
	m.DateOfBirth = dob
	m.MembershipEndDate = end
	return nil
}

// MemberListParams extends ListParams with a membership-status filter.
type MemberListParams struct {
	ListParams
	Membership string     // active | expiring | expired | none
	TrainerID  *uuid.UUID // when set, only members with a PT package for this trainer
}

// ListMembers returns a page of members with computed membership info.
func ListMembers(ctx context.Context, q Querier, p MemberListParams) (Page[Member], error) {
	p.Normalize()
	out := Page[Member]{Items: []Member{}, Page: p.Page, PageSize: p.PageSize}

	rows, err := q.Query(ctx, memberSelect+`
		SELECT `+memberCols+`, COUNT(*) OVER() AS total
		FROM base
		WHERE ($1 = '' OR full_name ILIKE $2 OR email ILIKE $2 OR phone ILIKE $2)
		  AND ($3 = '' OR status = $3)
		  AND ($4 = '' OR membership_status = $4)
		  AND ($7::uuid IS NULL OR EXISTS (
		        SELECT 1 FROM pt_packages pk WHERE pk.member_id = base.id AND pk.trainer_id = $7
		      ))
		ORDER BY full_name, id
		LIMIT $5 OFFSET $6`,
		p.Q, like(p.Q), p.Status, p.Membership, p.PageSize, p.Offset(), p.TrainerID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var m Member
		var dob, end *Date
		if err := rows.Scan(&m.ID, &m.FullName, &m.Email, &m.Phone, &dob, &m.Gender, &m.JoinedAt, &m.Status,
			&m.EmergencyContact, &m.Notes, &m.CreatedAt, &m.UpdatedAt,
			&m.CurrentPlanName, &end, &m.MembershipStatus, &m.OutstandingDues, &out.Total); err != nil {
			return out, err
		}
		m.DateOfBirth = dob
		m.MembershipEndDate = end
		out.Items = append(out.Items, m)
	}
	return out, rows.Err()
}

// GetMember fetches one member with computed membership info.
func GetMember(ctx context.Context, q Querier, id uuid.UUID) (Member, error) {
	var m Member
	err := scanMember(q.QueryRow(ctx, memberSelect+`SELECT `+memberCols+` FROM base WHERE id = $1`, id), &m)
	if errors.Is(err, pgx.ErrNoRows) {
		return Member{}, ErrNotFound
	}
	return m, err
}

// MemberInput is the writable subset of a member.
type MemberInput struct {
	FullName         string
	Email            *string
	Phone            *string
	DateOfBirth      *Date
	Gender           *string
	JoinedAt         Date
	Status           string
	EmergencyContact *string
	Notes            string
}

// CreateMember inserts a member for the current tenant.
func CreateMember(ctx context.Context, q Querier, tenantID uuid.UUID, in MemberInput) (Member, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx, `
		INSERT INTO members (tenant_id, full_name, email, phone, date_of_birth, gender, joined_at, status, emergency_contact, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id`,
		tenantID, in.FullName, in.Email, in.Phone, in.DateOfBirth, in.Gender, in.JoinedAt, in.Status, in.EmergencyContact, in.Notes,
	).Scan(&id)
	if err != nil {
		return Member{}, err
	}
	return GetMember(ctx, q, id)
}

// UpdateMember replaces a member's editable fields.
func UpdateMember(ctx context.Context, q Querier, id uuid.UUID, in MemberInput) (Member, error) {
	tag, err := q.Exec(ctx, `
		UPDATE members SET full_name = $2, email = $3, phone = $4, date_of_birth = $5, gender = $6,
		  joined_at = $7, status = $8, emergency_contact = $9, notes = $10
		WHERE id = $1`,
		id, in.FullName, in.Email, in.Phone, in.DateOfBirth, in.Gender, in.JoinedAt, in.Status, in.EmergencyContact, in.Notes)
	if err != nil {
		return Member{}, err
	}
	if tag.RowsAffected() == 0 {
		return Member{}, ErrNotFound
	}
	return GetMember(ctx, q, id)
}

// DeleteMember removes a member and cascades to subscriptions, payments, PT.
func DeleteMember(ctx context.Context, q Querier, id uuid.UUID) error {
	tag, err := q.Exec(ctx, `DELETE FROM members WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
