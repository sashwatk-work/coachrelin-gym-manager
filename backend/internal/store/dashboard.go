package store

import (
	"context"

	"github.com/google/uuid"
)

// ExpiringMembership is a row in the dashboard "expiring soon" list.
type ExpiringMembership struct {
	MemberID   uuid.UUID `json:"member_id"`
	MemberName string    `json:"member_name"`
	PlanName   string    `json:"plan_name"`
	EndDate    Date      `json:"end_date"`
	DaysLeft   int       `json:"days_left"`
}

// RevenuePoint is monthly revenue for the trend chart.
type RevenuePoint struct {
	Month  string  `json:"month"` // YYYY-MM
	Amount float64 `json:"amount"`
}

// Dashboard is the owner's overview.
type Dashboard struct {
	Today string `json:"today"` // YYYY-MM-DD in the tenant timezone

	TotalMembers        int `json:"total_members"`
	ActiveMembers       int `json:"active_members"`
	ActiveMemberships   int `json:"active_memberships"`
	ExpiringSoon        int `json:"expiring_soon"`
	ExpiredMemberships  int `json:"expired_memberships"`
	NewMembersThisMonth int `json:"new_members_this_month"`

	RevenueThisMonth float64 `json:"revenue_this_month"`
	RevenueLastMonth float64 `json:"revenue_last_month"`
	OutstandingDues  float64 `json:"outstanding_dues"`

	ActiveTrainers    int `json:"active_trainers"`
	ActivePTPackages  int `json:"active_pt_packages"`
	SessionsToday     int `json:"sessions_today"`
	SessionsThisWeek  int `json:"sessions_this_week"`
	SessionsCompleted int `json:"sessions_completed_this_month"`

	ExpiringMemberships []ExpiringMembership `json:"expiring_memberships"`
	TodaySessions       []PTSession          `json:"today_sessions"`
	RecentPayments      []Payment            `json:"recent_payments"`
	RevenueTrend        []RevenuePoint       `json:"revenue_trend"`
}

// GetDashboard aggregates the overview numbers for the current tenant.
func GetDashboard(ctx context.Context, q Querier) (Dashboard, error) {
	var d Dashboard
	var today Date

	err := q.QueryRow(ctx, `
		WITH latest AS (
		  SELECT DISTINCT ON (s.member_id) s.member_id, s.end_date
		  FROM subscriptions s
		  WHERE s.status <> 'cancelled'
		  ORDER BY s.member_id, (s.end_date >= CURRENT_DATE) DESC, s.end_date DESC
		)
		SELECT
		  CURRENT_DATE,
		  (SELECT COUNT(*) FROM members),
		  (SELECT COUNT(*) FROM members WHERE status = 'active'),
		  (SELECT COUNT(*) FROM latest WHERE end_date >= CURRENT_DATE),
		  (SELECT COUNT(*) FROM latest WHERE end_date >= CURRENT_DATE AND end_date <= CURRENT_DATE + 7),
		  (SELECT COUNT(*) FROM latest WHERE end_date < CURRENT_DATE),
		  (SELECT COUNT(*) FROM members WHERE joined_at >= date_trunc('month', CURRENT_DATE)::date),
		  (SELECT COALESCE(SUM(amount), 0)::float8 FROM payments WHERE paid_at >= date_trunc('month', now())),
		  (SELECT COALESCE(SUM(amount), 0)::float8 FROM payments
		     WHERE paid_at >= date_trunc('month', now()) - interval '1 month' AND paid_at < date_trunc('month', now())),
		  -- Outstanding dues: per member, billed items minus linked payments
		  -- minus unlinked ("General") payments, floored at zero, then summed.
		  (SELECT COALESCE(SUM(GREATEST(per.remaining - member_unlinked_credit(per.member_id), 0)), 0)::float8 FROM (
		     SELECT x.member_id, SUM(GREATEST(x.price - x.paid, 0)) AS remaining FROM (
		       SELECT s.member_id, s.price, COALESCE((SELECT SUM(amount) FROM payments p WHERE p.subscription_id = s.id), 0) AS paid
		       FROM subscriptions s WHERE s.status <> 'cancelled'
		       UNION ALL
		       SELECT pk.member_id, pk.price, COALESCE((SELECT SUM(amount) FROM payments p WHERE p.pt_package_id = pk.id), 0)
		       FROM pt_packages pk WHERE pk.status <> 'cancelled'
		     ) x GROUP BY x.member_id
		  ) per),
		  (SELECT COUNT(*) FROM trainers WHERE active),
		  (SELECT COUNT(*) FROM pt_packages WHERE status = 'active' AND (expires_at IS NULL OR expires_at >= CURRENT_DATE)),
		  (SELECT COUNT(*) FROM pt_sessions WHERE (scheduled_at AT TIME ZONE current_setting('TimeZone'))::date = CURRENT_DATE AND status <> 'cancelled'),
		  (SELECT COUNT(*) FROM pt_sessions WHERE (scheduled_at AT TIME ZONE current_setting('TimeZone'))::date BETWEEN CURRENT_DATE AND CURRENT_DATE + 6 AND status <> 'cancelled'),
		  (SELECT COUNT(*) FROM pt_sessions WHERE status = 'completed' AND scheduled_at >= date_trunc('month', now()))
	`).Scan(&today, &d.TotalMembers, &d.ActiveMembers, &d.ActiveMemberships, &d.ExpiringSoon, &d.ExpiredMemberships,
		&d.NewMembersThisMonth, &d.RevenueThisMonth, &d.RevenueLastMonth, &d.OutstandingDues,
		&d.ActiveTrainers, &d.ActivePTPackages, &d.SessionsToday, &d.SessionsThisWeek, &d.SessionsCompleted)
	if err != nil {
		return d, err
	}
	d.Today = today.String()

	// Expiring within 7 days.
	rows, err := q.Query(ctx, `
		SELECT DISTINCT ON (s.member_id) s.member_id, m.full_name, p.name, s.end_date, (s.end_date - CURRENT_DATE)::int
		FROM subscriptions s
		JOIN members m ON m.id = s.member_id
		JOIN membership_plans p ON p.id = s.plan_id
		WHERE s.status <> 'cancelled'
		ORDER BY s.member_id, (s.end_date >= CURRENT_DATE) DESC, s.end_date DESC`)
	if err != nil {
		return d, err
	}
	d.ExpiringMemberships = []ExpiringMembership{}
	for rows.Next() {
		var e ExpiringMembership
		if err := rows.Scan(&e.MemberID, &e.MemberName, &e.PlanName, &e.EndDate, &e.DaysLeft); err != nil {
			rows.Close()
			return d, err
		}
		if e.DaysLeft >= 0 && e.DaysLeft <= 7 {
			d.ExpiringMemberships = append(d.ExpiringMemberships, e)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return d, err
	}
	sortExpiring(d.ExpiringMemberships)

	// Today's sessions.
	todayD := today
	sessions, err := ListSessions(ctx, q, SessionListParams{
		ListParams: ListParams{PageSize: 50},
		From:       &todayD,
		To:         &todayD,
	})
	if err != nil {
		return d, err
	}
	d.TodaySessions = sessions.Items

	// Recent payments.
	payments, err := ListPayments(ctx, q, PaymentListParams{ListParams: ListParams{PageSize: 8}})
	if err != nil {
		return d, err
	}
	d.RecentPayments = payments.Items

	// Six-month revenue trend.
	trend, err := q.Query(ctx, `
		SELECT to_char(mon, 'YYYY-MM'),
		       COALESCE((SELECT SUM(amount) FROM payments p
		                 WHERE p.paid_at >= mon AND p.paid_at < mon + interval '1 month'), 0)::float8
		FROM generate_series(date_trunc('month', now()) - interval '5 months', date_trunc('month', now()), interval '1 month') AS mon
		ORDER BY mon`)
	if err != nil {
		return d, err
	}
	defer trend.Close()
	d.RevenueTrend = []RevenuePoint{}
	for trend.Next() {
		var p RevenuePoint
		if err := trend.Scan(&p.Month, &p.Amount); err != nil {
			return d, err
		}
		d.RevenueTrend = append(d.RevenueTrend, p)
	}
	return d, trend.Err()
}

// GetTrainerDashboard is the trainer's home: their clients and sessions only.
// Financial fields are left at zero so a trainer login cannot see gym revenue.
func GetTrainerDashboard(ctx context.Context, q Querier, trainerID uuid.UUID) (Dashboard, error) {
	var d Dashboard
	var today Date
	err := q.QueryRow(ctx, `
		SELECT
		  CURRENT_DATE,
		  (SELECT COUNT(DISTINCT member_id) FROM pt_packages WHERE trainer_id = $1 AND status <> 'cancelled'),
		  (SELECT COUNT(DISTINCT member_id) FROM pt_packages WHERE trainer_id = $1 AND status = 'active'
		     AND (expires_at IS NULL OR expires_at >= CURRENT_DATE)),
		  (SELECT COUNT(*) FROM pt_packages WHERE trainer_id = $1 AND status = 'active'
		     AND (expires_at IS NULL OR expires_at >= CURRENT_DATE)),
		  (SELECT COUNT(*) FROM pt_sessions WHERE trainer_id = $1
		     AND (scheduled_at AT TIME ZONE current_setting('TimeZone'))::date = CURRENT_DATE AND status <> 'cancelled'),
		  (SELECT COUNT(*) FROM pt_sessions WHERE trainer_id = $1
		     AND (scheduled_at AT TIME ZONE current_setting('TimeZone'))::date BETWEEN CURRENT_DATE AND CURRENT_DATE + 6
		     AND status <> 'cancelled'),
		  (SELECT COUNT(*) FROM pt_sessions WHERE trainer_id = $1 AND status = 'completed'
		     AND scheduled_at >= date_trunc('month', now()))
	`, trainerID).Scan(&today, &d.TotalMembers, &d.ActiveMembers, &d.ActivePTPackages,
		&d.SessionsToday, &d.SessionsThisWeek, &d.SessionsCompleted)
	if err != nil {
		return d, err
	}
	d.Today = today.String()
	d.ActiveTrainers = 1
	d.ExpiringMemberships = []ExpiringMembership{}
	d.RecentPayments = []Payment{}
	d.RevenueTrend = []RevenuePoint{}
	todayD := today
	sessions, err := ListSessions(ctx, q, SessionListParams{
		ListParams: ListParams{PageSize: 50},
		TrainerID:  &trainerID,
		From:       &todayD,
		To:         &todayD,
	})
	if err != nil {
		return d, err
	}
	d.TodaySessions = sessions.Items
	return d, nil
}

func sortExpiring(xs []ExpiringMembership) {
	// insertion sort by days left; lists are tiny
	for i := 1; i < len(xs); i++ {
		for j := i; j > 0 && xs[j].DaysLeft < xs[j-1].DaysLeft; j-- {
			xs[j], xs[j-1] = xs[j-1], xs[j]
		}
	}
}
