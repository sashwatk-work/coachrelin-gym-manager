// Package store contains the SQL repositories. Every function that touches a
// tenant-scoped table takes a pgx.Tx obtained from db.WithTenantTx so that the
// RLS tenant setting is always present.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// Querier is satisfied by pgx.Tx and *pgxpool.Pool.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Sentinel errors mapped to HTTP statuses by the API layer.
var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

// Date is a calendar date (no time component) serialised as YYYY-MM-DD.
type Date struct{ time.Time }

// NewDate builds a Date from y/m/d.
func NewDate(y int, m time.Month, d int) Date {
	return Date{time.Date(y, m, d, 0, 0, 0, 0, time.UTC)}
}

// DateOf truncates t to a date.
func DateOf(t time.Time) Date {
	return NewDate(t.Year(), t.Month(), t.Day())
}

// ParseDate parses YYYY-MM-DD.
func ParseDate(s string) (Date, error) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(s))
	if err != nil {
		return Date{}, fmt.Errorf("invalid date %q, expected YYYY-MM-DD", s)
	}
	return Date{t}, nil
}

// String formats as YYYY-MM-DD.
func (d Date) String() string { return d.Format("2006-01-02") }

// AddDays returns the date shifted by n days.
func (d Date) AddDays(n int) Date { return Date{d.AddDate(0, 0, n)} }

// MarshalJSON implements json.Marshaler.
func (d Date) MarshalJSON() ([]byte, error) { return json.Marshal(d.String()) }

// UnmarshalJSON implements json.Unmarshaler.
func (d *Date) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if s == "" {
		*d = Date{}
		return nil
	}
	p, err := ParseDate(s)
	if err != nil {
		return err
	}
	*d = p
	return nil
}

// ScanDate implements pgtype.DateScanner.
func (d *Date) ScanDate(v pgtype.Date) error {
	if !v.Valid {
		*d = Date{}
		return nil
	}
	*d = Date{v.Time}
	return nil
}

// DateValue implements pgtype.DateValuer.
func (d Date) DateValue() (pgtype.Date, error) {
	if d.IsZero() {
		return pgtype.Date{}, nil
	}
	return pgtype.Date{Time: d.Time, Valid: true}, nil
}

// ---------------------------------------------------------------------------
// Platform
// ---------------------------------------------------------------------------

// Tenant is a gym.
type Tenant struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Slug       string    `json:"slug"`
	Timezone   string    `json:"timezone"`
	Currency   string    `json:"currency"`
	Gstin      *string   `json:"gstin"`
	GstPercent float64   `json:"gst_percent"`
	CreatedAt  time.Time `json:"created_at"`
}

// Login roles. Members are not users — they do not sign in. A user may hold
// several roles; permissions are the union of those roles.
const (
	RoleOwner         = "owner"
	RoleReceptionist  = "receptionist"
	RoleTrainer       = "trainer"
	RoleCashierLegacy = "cashier" // accepted on input, stored as receptionist
)

// AllRoles is the canonical order used in JWT claims and the UI.
var AllRoles = []string{RoleOwner, RoleReceptionist, RoleTrainer}

// User is a gym login belonging to exactly one tenant.
type User struct {
	ID           uuid.UUID  `json:"id"`
	TenantID     uuid.UUID  `json:"tenant_id"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	FullName     string     `json:"full_name"`
	Roles        []string   `json:"roles"`
	TrainerID    *uuid.UUID `json:"trainer_id"`
	TrainerName  *string    `json:"trainer_name,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// HasRole reports whether the login includes this role.
func (u User) HasRole(role string) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// NormalizeRoles maps cashier→receptionist, drops blanks, de-duplicates and
// orders owner, receptionist, trainer. Unknown names return an error.
func NormalizeRoles(in []string) ([]string, error) {
	seen := map[string]struct{}{}
	for _, raw := range in {
		r := strings.ToLower(strings.TrimSpace(raw))
		if r == "" {
			continue
		}
		if r == RoleCashierLegacy {
			r = RoleReceptionist
		}
		switch r {
		case RoleOwner, RoleReceptionist, RoleTrainer:
			seen[r] = struct{}{}
		default:
			return nil, fmt.Errorf("unknown role %q", raw)
		}
	}
	out := make([]string, 0, len(AllRoles))
	for _, r := range AllRoles {
		if _, ok := seen[r]; ok {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("at least one role is required")
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Domain
// ---------------------------------------------------------------------------

// Trainer is a personal trainer employed by the gym.
type Trainer struct {
	ID          uuid.UUID `json:"id"`
	FullName    string    `json:"full_name"`
	Email       *string   `json:"email"`
	Phone       *string   `json:"phone"`
	Specialties []string  `json:"specialties"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Computed in list/detail queries.
	ActiveClients    int `json:"active_clients"`
	UpcomingSessions int `json:"upcoming_sessions"`
}

// MembershipPlan is a purchasable membership tier.
type MembershipPlan struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	DurationDays int       `json:"duration_days"`
	Price        float64   `json:"price"`
	Description  string    `json:"description"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	ActiveSubscriptions int `json:"active_subscriptions"`
}

// PTPlan is a reusable personal-training package template (the PT
// counterpart of MembershipPlan). Packages sold from it keep their own copy
// of sessions/price/expiry so they can be customised per member.
type PTPlan struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Sessions     int       `json:"sessions"`
	ValidityDays *int      `json:"validity_days"` // nil = never expires
	Price        float64   `json:"price"`
	Description  string    `json:"description"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	ActivePackages int `json:"active_packages"`
}

// Member is a gym member.
type Member struct {
	ID               uuid.UUID `json:"id"`
	FullName         string    `json:"full_name"`
	Email            *string   `json:"email"`
	Phone            *string   `json:"phone"`
	DateOfBirth      *Date     `json:"date_of_birth"`
	Gender           *string   `json:"gender"`
	JoinedAt         Date      `json:"joined_at"`
	Status           string    `json:"status"`
	EmergencyContact *string   `json:"emergency_contact"`
	Notes            string    `json:"notes"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`

	// Computed from the latest subscription.
	CurrentPlanName   *string `json:"current_plan_name"`
	MembershipEndDate *Date   `json:"membership_end_date"`
	MembershipStatus  string  `json:"membership_status"` // active | expiring | expired | none
	OutstandingDues   float64 `json:"outstanding_dues"`
}

// Subscription is a member's purchase of a plan for a period.
type Subscription struct {
	ID        uuid.UUID `json:"id"`
	MemberID  uuid.UUID `json:"member_id"`
	PlanID    uuid.UUID `json:"plan_id"`
	PlanName  string    `json:"plan_name"`
	StartDate Date      `json:"start_date"`
	EndDate   Date      `json:"end_date"`
	Price     float64   `json:"price"`
	Paid      float64   `json:"paid"`
	Status    string    `json:"status"` // effective status (time-aware)
	CreatedAt time.Time `json:"created_at"`
}

// Payment is money received from a member.
type Payment struct {
	ID             uuid.UUID  `json:"id"`
	MemberID       uuid.UUID  `json:"member_id"`
	MemberName     string     `json:"member_name"`
	SubscriptionID *uuid.UUID `json:"subscription_id"`
	PTPackageID    *uuid.UUID `json:"pt_package_id"`
	Amount         float64    `json:"amount"`
	Method         string     `json:"method"`
	PaidAt         time.Time  `json:"paid_at"`
	Notes          string     `json:"notes"`
	CreatedAt      time.Time  `json:"created_at"`
	ReferenceLabel string     `json:"reference_label"` // "Monthly plan" / "10-session PT"
}

// PTPackage is a block of personal-training sessions bought by a member.
type PTPackage struct {
	ID                uuid.UUID  `json:"id"`
	MemberID          uuid.UUID  `json:"member_id"`
	MemberName        string     `json:"member_name"`
	TrainerID         uuid.UUID  `json:"trainer_id"`
	TrainerName       string     `json:"trainer_name"`
	PTPlanID          *uuid.UUID `json:"pt_plan_id"`   // nil = custom package
	PTPlanName        *string    `json:"pt_plan_name"` // name of the plan it was bought from
	Name              string     `json:"name"`
	TotalSessions     int        `json:"total_sessions"`
	SessionsUsed      int        `json:"sessions_used"`      // completed + no_show
	SessionsScheduled int        `json:"sessions_scheduled"` // future, still scheduled
	SessionsRemaining int        `json:"sessions_remaining"` // total - used
	Price             float64    `json:"price"`
	Paid              float64    `json:"paid"`
	PurchasedAt       Date       `json:"purchased_at"`
	ExpiresAt         *Date      `json:"expires_at"`
	Status            string     `json:"status"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// PTSession is a single training appointment drawn from a package.
type PTSession struct {
	ID              uuid.UUID `json:"id"`
	PackageID       uuid.UUID `json:"package_id"`
	PackageName     string    `json:"package_name"`
	MemberID        uuid.UUID `json:"member_id"`
	MemberName      string    `json:"member_name"`
	TrainerID       uuid.UUID `json:"trainer_id"`
	TrainerName     string    `json:"trainer_name"`
	ScheduledAt     time.Time `json:"scheduled_at"`
	DurationMinutes int       `json:"duration_minutes"`
	Status          string    `json:"status"`
	Notes           string    `json:"notes"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ---------------------------------------------------------------------------
// Listing helpers
// ---------------------------------------------------------------------------

// ListParams are the common pagination/search params.
type ListParams struct {
	Q        string
	Status   string
	Page     int
	PageSize int
}

// Normalize clamps paging values.
func (p *ListParams) Normalize() {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 {
		p.PageSize = 25
	}
	if p.PageSize > 200 {
		p.PageSize = 200
	}
	p.Q = strings.TrimSpace(p.Q)
}

// Offset is the SQL offset for the page.
func (p ListParams) Offset() int { return (p.Page - 1) * p.PageSize }

// Page is a paginated result.
type Page[T any] struct {
	Items    []T `json:"items"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

// isUniqueViolation reports whether err is a Postgres unique constraint error.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// isFKViolation reports whether err is a Postgres foreign-key error.
func isFKViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

// isRestrictViolation reports whether err is a FK RESTRICT error on delete.
func isRestrictViolation(err error) bool {
	return isFKViolation(err)
}

// like wraps a search term for ILIKE.
func like(q string) string { return "%" + q + "%" }
