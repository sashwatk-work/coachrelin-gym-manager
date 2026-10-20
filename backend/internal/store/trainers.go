package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const trainerSelect = `
SELECT t.id, t.full_name, t.email, t.phone, t.specialties, t.active, t.created_at, t.updated_at,
  (SELECT COUNT(DISTINCT pk.member_id) FROM pt_packages pk WHERE pk.trainer_id = t.id AND pk.status = 'active') AS active_clients,
  (SELECT COUNT(*) FROM pt_sessions s WHERE s.trainer_id = t.id AND s.status = 'scheduled' AND s.scheduled_at >= now()) AS upcoming_sessions
FROM trainers t`

func scanTrainer(row pgx.Row, t *Trainer) error {
	return row.Scan(&t.ID, &t.FullName, &t.Email, &t.Phone, &t.Specialties, &t.Active, &t.CreatedAt, &t.UpdatedAt,
		&t.ActiveClients, &t.UpcomingSessions)
}

// ListTrainers returns trainers, optionally filtered by search and active flag
// (Status = "active" | "inactive").
func ListTrainers(ctx context.Context, q Querier, p ListParams) (Page[Trainer], error) {
	p.Normalize()
	out := Page[Trainer]{Items: []Trainer{}, Page: p.Page, PageSize: p.PageSize}
	rows, err := q.Query(ctx, trainerSelect+`
		WHERE ($1 = '' OR t.full_name ILIKE $2 OR t.email ILIKE $2 OR t.phone ILIKE $2)
		  AND ($3 = '' OR ($3 = 'active' AND t.active) OR ($3 = 'inactive' AND NOT t.active))
		ORDER BY t.active DESC, t.full_name, t.id
		LIMIT $4 OFFSET $5`,
		p.Q, like(p.Q), p.Status, p.PageSize, p.Offset())
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var t Trainer
		if err := scanTrainer(rows, &t); err != nil {
			return out, err
		}
		out.Items = append(out.Items, t)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	if err := q.QueryRow(ctx, `
		SELECT COUNT(*) FROM trainers t
		WHERE ($1 = '' OR t.full_name ILIKE $2 OR t.email ILIKE $2 OR t.phone ILIKE $2)
		  AND ($3 = '' OR ($3 = 'active' AND t.active) OR ($3 = 'inactive' AND NOT t.active))`,
		p.Q, like(p.Q), p.Status).Scan(&out.Total); err != nil {
		return out, err
	}
	return out, nil
}

// GetTrainer fetches one trainer.
func GetTrainer(ctx context.Context, q Querier, id uuid.UUID) (Trainer, error) {
	var t Trainer
	err := scanTrainer(q.QueryRow(ctx, trainerSelect+` WHERE t.id = $1`, id), &t)
	if errors.Is(err, pgx.ErrNoRows) {
		return Trainer{}, ErrNotFound
	}
	return t, err
}

// TrainerInput is the writable subset of a trainer.
type TrainerInput struct {
	FullName    string
	Email       *string
	Phone       *string
	Specialties []string
	Active      bool
}

// CreateTrainer inserts a trainer.
func CreateTrainer(ctx context.Context, q Querier, tenantID uuid.UUID, in TrainerInput) (Trainer, error) {
	if in.Specialties == nil {
		in.Specialties = []string{}
	}
	var id uuid.UUID
	err := q.QueryRow(ctx, `
		INSERT INTO trainers (tenant_id, full_name, email, phone, specialties, active)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		tenantID, in.FullName, in.Email, in.Phone, in.Specialties, in.Active,
	).Scan(&id)
	if err != nil {
		return Trainer{}, err
	}
	return GetTrainer(ctx, q, id)
}

// UpdateTrainer replaces a trainer's editable fields.
func UpdateTrainer(ctx context.Context, q Querier, id uuid.UUID, in TrainerInput) (Trainer, error) {
	if in.Specialties == nil {
		in.Specialties = []string{}
	}
	tag, err := q.Exec(ctx, `
		UPDATE trainers SET full_name = $2, email = $3, phone = $4, specialties = $5, active = $6
		WHERE id = $1`,
		id, in.FullName, in.Email, in.Phone, in.Specialties, in.Active)
	if err != nil {
		return Trainer{}, err
	}
	if tag.RowsAffected() == 0 {
		return Trainer{}, ErrNotFound
	}
	return GetTrainer(ctx, q, id)
}

// DeleteTrainer removes a trainer. Fails with ErrConflict if the trainer has
// packages or sessions (FK RESTRICT); deactivate instead.
func DeleteTrainer(ctx context.Context, q Querier, id uuid.UUID) error {
	tag, err := q.Exec(ctx, `DELETE FROM trainers WHERE id = $1`, id)
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
