package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrNoSessionsLeft is returned when scheduling would exceed a package.
var ErrNoSessionsLeft = errors.New("no sessions left in this package")

// ErrPackageNotActive is returned when scheduling against a closed package.
var ErrPackageNotActive = errors.New("package is not active")

// ---------------------------------------------------------------------------
// Packages
// ---------------------------------------------------------------------------

const packageSelect = `
SELECT pk.id, pk.member_id, m.full_name, pk.trainer_id, t.full_name, pk.pt_plan_id, pl.name, pk.name, pk.total_sessions,
  (SELECT COUNT(*) FROM pt_sessions s WHERE s.package_id = pk.id AND s.status IN ('completed', 'no_show'))::int AS used,
  (SELECT COUNT(*) FROM pt_sessions s WHERE s.package_id = pk.id AND s.status = 'scheduled')::int AS scheduled,
  pk.price::float8,
  (lp.paid + CASE WHEN pk.status = 'cancelled' THEN 0
                  ELSE allocated_credit(pk.member_id, pk.created_at, pk.id, pk.price - lp.paid) END)::float8 AS paid,
  pk.purchased_at, pk.expires_at,
  CASE WHEN pk.status = 'active' AND pk.expires_at IS NOT NULL AND pk.expires_at < CURRENT_DATE
       THEN 'expired' ELSE pk.status::text END AS status,
  pk.created_at, pk.updated_at
FROM pt_packages pk
JOIN members m ON m.id = pk.member_id
JOIN trainers t ON t.id = pk.trainer_id
LEFT JOIN pt_plans pl ON pl.id = pk.pt_plan_id
CROSS JOIN LATERAL (
  SELECT COALESCE((SELECT SUM(amount) FROM payments pay WHERE pay.pt_package_id = pk.id), 0) AS paid
) lp`

func scanPackage(row pgx.Row, p *PTPackage) error {
	var exp *Date
	err := row.Scan(&p.ID, &p.MemberID, &p.MemberName, &p.TrainerID, &p.TrainerName, &p.PTPlanID, &p.PTPlanName, &p.Name, &p.TotalSessions,
		&p.SessionsUsed, &p.SessionsScheduled, &p.Price, &p.Paid, &p.PurchasedAt, &exp, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return err
	}
	p.ExpiresAt = exp
	p.SessionsRemaining = p.TotalSessions - p.SessionsUsed
	if p.SessionsRemaining < 0 {
		p.SessionsRemaining = 0
	}
	return nil
}

// PackageListParams filters packages.
type PackageListParams struct {
	ListParams
	MemberID  *uuid.UUID
	TrainerID *uuid.UUID
}

// ListPackages returns a filtered page of packages.
func ListPackages(ctx context.Context, q Querier, p PackageListParams) (Page[PTPackage], error) {
	p.Normalize()
	out := Page[PTPackage]{Items: []PTPackage{}, Page: p.Page, PageSize: p.PageSize}

	// Status filter is applied on the effective status, so wrap in a subquery.
	rows, err := q.Query(ctx, `
		SELECT * FROM (`+packageSelect+`
		  WHERE ($1 = '' OR m.full_name ILIKE $2 OR t.full_name ILIKE $2 OR pk.name ILIKE $2)
		    AND ($3::uuid IS NULL OR pk.member_id = $3)
		    AND ($4::uuid IS NULL OR pk.trainer_id = $4)
		) x
		WHERE ($5 = '' OR x.status = $5)
		ORDER BY (x.status = 'active') DESC, x.purchased_at DESC, x.created_at DESC
		LIMIT $6 OFFSET $7`,
		p.Q, like(p.Q), p.MemberID, p.TrainerID, p.Status, p.PageSize, p.Offset())
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var pk PTPackage
		if err := scanPackage(rows, &pk); err != nil {
			return out, err
		}
		out.Items = append(out.Items, pk)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	err = q.QueryRow(ctx, `
		SELECT COUNT(*) FROM (`+packageSelect+`
		  WHERE ($1 = '' OR m.full_name ILIKE $2 OR t.full_name ILIKE $2 OR pk.name ILIKE $2)
		    AND ($3::uuid IS NULL OR pk.member_id = $3)
		    AND ($4::uuid IS NULL OR pk.trainer_id = $4)
		) x WHERE ($5 = '' OR x.status = $5)`,
		p.Q, like(p.Q), p.MemberID, p.TrainerID, p.Status).Scan(&out.Total)
	return out, err
}

// GetPackage fetches one package.
func GetPackage(ctx context.Context, q Querier, id uuid.UUID) (PTPackage, error) {
	var pk PTPackage
	err := scanPackage(q.QueryRow(ctx, packageSelect+` WHERE pk.id = $1`, id), &pk)
	if errors.Is(err, pgx.ErrNoRows) {
		return PTPackage{}, ErrNotFound
	}
	return pk, err
}

// PackageInput is the writable subset of a package.
type PackageInput struct {
	MemberID       uuid.UUID
	TrainerID      uuid.UUID
	PTPlanID       *uuid.UUID // plan the package was bought from; nil for a custom package
	Name           string
	TotalSessions  int
	Price          float64
	PurchasedAt    Date
	ExpiresAt      *Date
	SkipPlanExpiry bool // true = leave ExpiresAt as sent, even if the plan has a validity
	Status         string
}

// ApplyPlan links the package to a PT plan and fills in whatever the caller
// left unset from it: name, sessions and expiry (purchased_at + validity).
// Price comes from the plan unless an explicit override is given (0 is a
// valid override, hence the pointer). This lets a caller say just "this plan
// for this member" and customise only the parts they want.
func (in *PackageInput) ApplyPlan(p PTPlan, priceOverride *float64) {
	in.PTPlanID = &p.ID
	if in.Name == "" {
		in.Name = p.Name
	}
	if in.TotalSessions <= 0 {
		in.TotalSessions = p.Sessions
	}
	if priceOverride != nil {
		in.Price = *priceOverride
	} else {
		in.Price = p.Price
	}
	if in.ExpiresAt == nil && p.ValidityDays != nil && !in.SkipPlanExpiry {
		exp := in.PurchasedAt.AddDays(*p.ValidityDays)
		in.ExpiresAt = &exp
	}
}

// CreatePackage inserts a package.
func CreatePackage(ctx context.Context, q Querier, tenantID uuid.UUID, in PackageInput) (PTPackage, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx, `
		INSERT INTO pt_packages (tenant_id, member_id, trainer_id, pt_plan_id, name, total_sessions, price, purchased_at, expires_at, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id`,
		tenantID, in.MemberID, in.TrainerID, in.PTPlanID, in.Name, in.TotalSessions, in.Price, in.PurchasedAt, in.ExpiresAt, in.Status,
	).Scan(&id)
	if err != nil {
		if isFKViolation(err) {
			return PTPackage{}, ErrNotFound
		}
		return PTPackage{}, err
	}
	return GetPackage(ctx, q, id)
}

// UpdatePackage replaces a package's editable fields. Member cannot change.
func UpdatePackage(ctx context.Context, q Querier, id uuid.UUID, in PackageInput) (PTPackage, error) {
	tag, err := q.Exec(ctx, `
		UPDATE pt_packages SET trainer_id = $2, name = $3, total_sessions = $4, price = $5,
		  purchased_at = $6, expires_at = $7, status = $8, pt_plan_id = $9
		WHERE id = $1`,
		id, in.TrainerID, in.Name, in.TotalSessions, in.Price, in.PurchasedAt, in.ExpiresAt, in.Status, in.PTPlanID)
	if err != nil {
		if isFKViolation(err) {
			return PTPackage{}, ErrNotFound
		}
		return PTPackage{}, err
	}
	if tag.RowsAffected() == 0 {
		return PTPackage{}, ErrNotFound
	}
	if err := reconcilePackageStatus(ctx, q, id); err != nil {
		return PTPackage{}, err
	}
	return GetPackage(ctx, q, id)
}

// DeletePackage removes a package and its sessions.
func DeletePackage(ctx context.Context, q Querier, id uuid.UUID) error {
	tag, err := q.Exec(ctx, `DELETE FROM pt_packages WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListMemberPackages returns all packages for a member.
func ListMemberPackages(ctx context.Context, q Querier, memberID uuid.UUID) ([]PTPackage, error) {
	rows, err := q.Query(ctx, packageSelect+` WHERE pk.member_id = $1 ORDER BY (pk.status = 'active') DESC, pk.purchased_at DESC`, memberID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PTPackage{}
	for rows.Next() {
		var pk PTPackage
		if err := scanPackage(rows, &pk); err != nil {
			return nil, err
		}
		out = append(out, pk)
	}
	return out, rows.Err()
}

// reconcilePackageStatus flips a package between active and completed based
// on how many sessions have been consumed.
func reconcilePackageStatus(ctx context.Context, q Querier, packageID uuid.UUID) error {
	_, err := q.Exec(ctx, `
		UPDATE pt_packages pk SET status = CASE
		  WHEN pk.status = 'active' AND used.n >= pk.total_sessions THEN 'completed'::pt_package_status
		  WHEN pk.status = 'completed' AND used.n < pk.total_sessions THEN 'active'::pt_package_status
		  ELSE pk.status END
		FROM (SELECT COUNT(*) AS n FROM pt_sessions s WHERE s.package_id = $1 AND s.status IN ('completed', 'no_show')) used
		WHERE pk.id = $1`, packageID)
	return err
}

// ---------------------------------------------------------------------------
// Sessions
// ---------------------------------------------------------------------------

const sessionSelect = `
SELECT s.id, s.package_id, pk.name, s.member_id, m.full_name, s.trainer_id, t.full_name,
  s.scheduled_at, s.duration_minutes, s.status::text, s.notes, s.created_at, s.updated_at
FROM pt_sessions s
JOIN pt_packages pk ON pk.id = s.package_id
JOIN members m ON m.id = s.member_id
JOIN trainers t ON t.id = s.trainer_id`

func scanSession(row pgx.Row, s *PTSession) error {
	return row.Scan(&s.ID, &s.PackageID, &s.PackageName, &s.MemberID, &s.MemberName, &s.TrainerID, &s.TrainerName,
		&s.ScheduledAt, &s.DurationMinutes, &s.Status, &s.Notes, &s.CreatedAt, &s.UpdatedAt)
}

// SessionListParams filters sessions.
type SessionListParams struct {
	ListParams
	MemberID  *uuid.UUID
	TrainerID *uuid.UUID
	PackageID *uuid.UUID
	From      *Date
	To        *Date
	Desc      bool
}

// ListSessions returns a filtered page of sessions.
func ListSessions(ctx context.Context, q Querier, p SessionListParams) (Page[PTSession], error) {
	p.Normalize()
	out := Page[PTSession]{Items: []PTSession{}, Page: p.Page, PageSize: p.PageSize}

	where := `
		WHERE ($1 = '' OR m.full_name ILIKE $2 OR t.full_name ILIKE $2)
		  AND ($3::uuid IS NULL OR s.member_id = $3)
		  AND ($4::uuid IS NULL OR s.trainer_id = $4)
		  AND ($5::uuid IS NULL OR s.package_id = $5)
		  AND ($6 = '' OR s.status::text = $6)
		  AND ($7::date IS NULL OR (s.scheduled_at AT TIME ZONE current_setting('TimeZone'))::date >= $7)
		  AND ($8::date IS NULL OR (s.scheduled_at AT TIME ZONE current_setting('TimeZone'))::date <= $8)`
	args := []any{p.Q, like(p.Q), p.MemberID, p.TrainerID, p.PackageID, p.Status, p.From, p.To}

	order := " ORDER BY s.scheduled_at ASC, s.created_at ASC"
	if p.Desc {
		order = " ORDER BY s.scheduled_at DESC, s.created_at DESC"
	}
	rows, err := q.Query(ctx, sessionSelect+where+order+` LIMIT $9 OFFSET $10`, append(args, p.PageSize, p.Offset())...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var s PTSession
		if err := scanSession(rows, &s); err != nil {
			return out, err
		}
		out.Items = append(out.Items, s)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	err = q.QueryRow(ctx, `SELECT COUNT(*) FROM pt_sessions s
		JOIN pt_packages pk ON pk.id = s.package_id
		JOIN members m ON m.id = s.member_id
		JOIN trainers t ON t.id = s.trainer_id`+where, args...).Scan(&out.Total)
	return out, err
}

// GetSession fetches one session.
func GetSession(ctx context.Context, q Querier, id uuid.UUID) (PTSession, error) {
	var s PTSession
	err := scanSession(q.QueryRow(ctx, sessionSelect+` WHERE s.id = $1`, id), &s)
	if errors.Is(err, pgx.ErrNoRows) {
		return PTSession{}, ErrNotFound
	}
	return s, err
}

// SessionInput is the writable subset of a session.
type SessionInput struct {
	PackageID       uuid.UUID
	TrainerID       *uuid.UUID // nil = use the package's trainer
	ScheduledAt     time.Time
	DurationMinutes int
	Status          string
	Notes           string
}

// CreateSession schedules a session against a package. It fails if the
// package is not active or has no sessions left (used + scheduled >= total).
func CreateSession(ctx context.Context, q Querier, tenantID uuid.UUID, in SessionInput) (PTSession, error) {
	pk, err := GetPackage(ctx, q, in.PackageID)
	if err != nil {
		return PTSession{}, err
	}
	if pk.Status != "active" {
		return PTSession{}, ErrPackageNotActive
	}
	if pk.SessionsUsed+pk.SessionsScheduled >= pk.TotalSessions {
		return PTSession{}, ErrNoSessionsLeft
	}
	trainerID := pk.TrainerID
	if in.TrainerID != nil {
		trainerID = *in.TrainerID
	}
	if in.DurationMinutes <= 0 {
		in.DurationMinutes = 60
	}
	if in.Status == "" {
		in.Status = "scheduled"
	}
	var id uuid.UUID
	err = q.QueryRow(ctx, `
		INSERT INTO pt_sessions (tenant_id, package_id, member_id, trainer_id, scheduled_at, duration_minutes, status, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		tenantID, pk.ID, pk.MemberID, trainerID, in.ScheduledAt, in.DurationMinutes, in.Status, in.Notes,
	).Scan(&id)
	if err != nil {
		if isFKViolation(err) {
			return PTSession{}, ErrNotFound
		}
		return PTSession{}, err
	}
	if err := reconcilePackageStatus(ctx, q, pk.ID); err != nil {
		return PTSession{}, err
	}
	return GetSession(ctx, q, id)
}

// UpdateSession edits schedule/trainer/notes/status of a session.
func UpdateSession(ctx context.Context, q Querier, id uuid.UUID, trainerID uuid.UUID, scheduledAt time.Time, duration int, status, notes string) (PTSession, error) {
	var packageID uuid.UUID
	err := q.QueryRow(ctx, `
		UPDATE pt_sessions SET trainer_id = $2, scheduled_at = $3, duration_minutes = $4, status = $5, notes = $6
		WHERE id = $1 RETURNING package_id`,
		id, trainerID, scheduledAt, duration, status, notes).Scan(&packageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PTSession{}, ErrNotFound
		}
		if isFKViolation(err) {
			return PTSession{}, ErrNotFound
		}
		return PTSession{}, err
	}
	if err := reconcilePackageStatus(ctx, q, packageID); err != nil {
		return PTSession{}, err
	}
	return GetSession(ctx, q, id)
}

// SetSessionStatus changes only the status (and optionally appends notes),
// then reconciles the parent package.
func SetSessionStatus(ctx context.Context, q Querier, id uuid.UUID, status string, notes *string) (PTSession, error) {
	var packageID uuid.UUID
	err := q.QueryRow(ctx, `
		UPDATE pt_sessions SET status = $2, notes = COALESCE($3, notes)
		WHERE id = $1 RETURNING package_id`, id, status, notes).Scan(&packageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PTSession{}, ErrNotFound
		}
		return PTSession{}, err
	}
	if err := reconcilePackageStatus(ctx, q, packageID); err != nil {
		return PTSession{}, err
	}
	return GetSession(ctx, q, id)
}

// DeleteSession removes a session and reconciles the package.
func DeleteSession(ctx context.Context, q Querier, id uuid.UUID) error {
	var packageID uuid.UUID
	err := q.QueryRow(ctx, `DELETE FROM pt_sessions WHERE id = $1 RETURNING package_id`, id).Scan(&packageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	return reconcilePackageStatus(ctx, q, packageID)
}
