package store

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const userSelect = `
SELECT u.id, u.tenant_id, u.email, u.password_hash, u.full_name, u.roles::text[], u.trainer_id, t.full_name, u.created_at
FROM users u
LEFT JOIN trainers t ON t.id = u.trainer_id`

const userSelectBare = `
SELECT id, tenant_id, email, password_hash, full_name, roles::text[], trainer_id, NULL::text, created_at
FROM users`

func scanUser(row pgx.Row, u *User) error {
	err := row.Scan(&u.ID, &u.TenantID, &u.Email, &u.PasswordHash, &u.FullName, &u.Roles, &u.TrainerID, &u.TrainerName, &u.CreatedAt)
	if u.Roles == nil {
		u.Roles = []string{}
	}
	return err
}

// CreateUser inserts a gym login. Emails are stored lowercase and are globally unique.
// Pass trainerID when roles include trainer; the insert must run in a tenant-scoped
// transaction so RLS can see the referenced trainers row.
func CreateUser(ctx context.Context, q Querier, tenantID uuid.UUID, email, passwordHash, fullName string, roles []string, trainerID *uuid.UUID) (User, error) {
	var u User
	err := scanUser(q.QueryRow(ctx, `
		INSERT INTO users (tenant_id, email, password_hash, full_name, roles, trainer_id)
		VALUES ($1, lower($2), $3, $4, $5::user_role[], $6)
		RETURNING id, tenant_id, email, password_hash, full_name, roles::text[], trainer_id,
		          (SELECT full_name FROM trainers WHERE id = trainer_id), created_at`,
		tenantID, strings.TrimSpace(email), passwordHash, fullName, roles, trainerID,
	), &u)
	if isUniqueViolation(err) {
		return User{}, ErrConflict
	}
	return u, err
}

// GetUserByEmail finds a user for login.
func GetUserByEmail(ctx context.Context, q Querier, email string) (User, error) {
	var u User
	err := scanUser(q.QueryRow(ctx, userSelectBare+` WHERE lower(email) = lower($1)`, strings.TrimSpace(email)), &u)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

// GetUser fetches a user scoped to a tenant.
func GetUser(ctx context.Context, q Querier, tenantID, id uuid.UUID) (User, error) {
	var u User
	err := scanUser(q.QueryRow(ctx, userSelectBare+` WHERE id = $1 AND tenant_id = $2`, id, tenantID), &u)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

// ListUsers returns every login for a tenant.
func ListUsers(ctx context.Context, q Querier, tenantID uuid.UUID) ([]User, error) {
	rows, err := q.Query(ctx, userSelect+` WHERE u.tenant_id = $1 ORDER BY ('owner' = ANY(u.roles)) DESC, u.full_name, u.id`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		if err := scanUser(rows, &u); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// UserUpdate is the writable subset of a user. PasswordHash is optional.
type UserUpdate struct {
	FullName     string
	Roles        []string
	TrainerID    *uuid.UUID
	PasswordHash *string
}

// UpdateUser changes name, roles, linked trainer, and optionally the password.
func UpdateUser(ctx context.Context, q Querier, tenantID, id uuid.UUID, in UserUpdate) (User, error) {
	var u User
	err := scanUser(q.QueryRow(ctx, `
		UPDATE users SET
		  full_name = $3,
		  roles = $4::user_role[],
		  trainer_id = $5,
		  password_hash = COALESCE($6, password_hash)
		WHERE id = $1 AND tenant_id = $2
		RETURNING id, tenant_id, email, password_hash, full_name, roles::text[], trainer_id,
		          (SELECT full_name FROM trainers WHERE id = trainer_id), created_at`,
		id, tenantID, in.FullName, in.Roles, in.TrainerID, in.PasswordHash,
	), &u)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if isUniqueViolation(err) {
		return User{}, ErrConflict
	}
	return u, err
}

// DeleteUser removes a login.
func DeleteUser(ctx context.Context, q Querier, tenantID, id uuid.UUID) error {
	tag, err := q.Exec(ctx, `DELETE FROM users WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CountOwners returns how many owner logins the tenant has.
func CountOwners(ctx context.Context, q Querier, tenantID uuid.UUID) (int, error) {
	var n int
	err := q.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE tenant_id = $1 AND 'owner' = ANY(roles)`, tenantID).Scan(&n)
	return n, err
}

// MemberHasTrainer is true when the member has (or had) a PT package with that trainer.
func MemberHasTrainer(ctx context.Context, q Querier, memberID, trainerID uuid.UUID) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, `
		SELECT EXISTS(
		  SELECT 1 FROM pt_packages WHERE member_id = $1 AND trainer_id = $2
		)`, memberID, trainerID).Scan(&ok)
	return ok, err
}
