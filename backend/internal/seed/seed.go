// Package seed bootstraps Coach Relin Fitness and, optionally, fills it with
// demo members for local development.
package seed

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/auth"
	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/db"
	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/store"
)

// Demo credentials.
const (
	TenantName        = "Coach Relin Fitness"
	TenantSlug        = "coach-relin-fitness"
	OwnerEmail        = "owner@coachrelin.fit"
	OwnerPassword     = "password123"
	OwnerName         = "Relin"
	ReceptionistEmail = "receptionist@coachrelin.fit"
	ReceptionistName  = "Desk"
	CashierEmail      = "cashier@coachrelin.fit" // older seeded logins
	TrainerEmail      = "trainer@coachrelin.fit"
	Timezone          = "Asia/Kolkata"
	Currency          = "INR"
)

// Result summarises what the seeder did.
type Result struct {
	Tenant     store.Tenant
	OwnerEmail string
	Created    bool // false if the tenant already existed and seeding was skipped
}

// EnsureGym creates the Coach Relin Fitness gym and Relin's owner login if
// they do not already exist. It does not add members or sample data.
func EnsureGym(ctx context.Context, database *db.DB) (Result, error) {
	existing, err := store.GetTenantBySlug(ctx, database.Pool, TenantSlug)
	if err == nil {
		if _, err := store.GetUserByEmail(ctx, database.Pool, OwnerEmail); errors.Is(err, store.ErrNotFound) {
			hash, err := auth.HashPassword(OwnerPassword)
			if err != nil {
				return Result{}, err
			}
			if err := database.WithTenantTx(ctx, existing.ID, func(tx pgx.Tx) error {
				_, err := store.CreateUser(ctx, tx, existing.ID, OwnerEmail, hash, OwnerName, []string{store.RoleOwner}, nil)
				return err
			}); err != nil {
				return Result{}, fmt.Errorf("create owner: %w", err)
			}
		} else if err != nil {
			return Result{}, err
		}
		return Result{Tenant: existing, OwnerEmail: OwnerEmail, Created: false}, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return Result{}, err
	}

	hash, err := auth.HashPassword(OwnerPassword)
	if err != nil {
		return Result{}, err
	}

	var tenant store.Tenant
	err = database.WithTx(ctx, func(tx pgx.Tx) error {
		t, err := store.CreateTenant(ctx, tx, TenantName, Timezone, Currency)
		if err != nil {
			return err
		}
		if _, err := store.CreateUser(ctx, tx, t.ID, OwnerEmail, hash, OwnerName, []string{store.RoleOwner}, nil); err != nil {
			return fmt.Errorf("create owner: %w", err)
		}
		tenant = t
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	return Result{Tenant: tenant, OwnerEmail: OwnerEmail, Created: true}, nil
}

// Run seeds Coach Relin Fitness with realistic demo members, plans, trainers,
// subscriptions, payments, PT packages and sessions. Idempotent: if the gym
// already has members, only missing staff logins are added.
func Run(ctx context.Context, database *db.DB) (Result, error) {
	res, err := EnsureGym(ctx, database)
	if err != nil {
		return Result{}, err
	}
	if err := ensureDemoLogins(ctx, database, res.Tenant.ID); err != nil {
		return Result{}, err
	}

	var n int64
	if err := database.WithTenantTx(ctx, res.Tenant.ID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM members`).Scan(&n)
	}); err != nil {
		return Result{}, err
	}
	if n > 0 {
		return res, nil
	}

	if err := database.WithTenantTx(ctx, res.Tenant.ID, func(tx pgx.Tx) error {
		return populate(ctx, tx, res.Tenant.ID)
	}); err != nil {
		return Result{}, fmt.Errorf("populate: %w", err)
	}
	return res, nil
}

func ensureDemoLogins(ctx context.Context, database *db.DB, tenantID uuid.UUID) error {
	hash, err := auth.HashPassword(OwnerPassword)
	if err != nil {
		return err
	}
	if _, err := store.GetUserByEmail(ctx, database.Pool, ReceptionistEmail); errors.Is(err, store.ErrNotFound) {
		if _, err := store.GetUserByEmail(ctx, database.Pool, CashierEmail); errors.Is(err, store.ErrNotFound) {
			if err := database.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
				_, err := store.CreateUser(ctx, tx, tenantID, ReceptionistEmail, hash, ReceptionistName, []string{store.RoleReceptionist}, nil)
				return err
			}); err != nil {
				return fmt.Errorf("demo receptionist: %w", err)
			}
		} else if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if _, err := store.GetUserByEmail(ctx, database.Pool, TrainerEmail); errors.Is(err, store.ErrNotFound) {
		if err := database.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
			trainers, err := store.ListTrainers(ctx, tx, store.ListParams{PageSize: 20})
			if err != nil {
				return err
			}
			var trainerID *uuid.UUID
			for i := range trainers.Items {
				if trainers.Items[i].FullName == "Arjun Nair" {
					id := trainers.Items[i].ID
					trainerID = &id
					break
				}
			}
			if trainerID == nil && len(trainers.Items) > 0 {
				id := trainers.Items[0].ID
				trainerID = &id
			}
			if trainerID == nil {
				return nil
			}
			_, err = store.CreateUser(ctx, tx, tenantID, TrainerEmail, hash, "Arjun Nair", []string{store.RoleTrainer}, trainerID)
			return err
		}); err != nil {
			return fmt.Errorf("demo trainer: %w", err)
		}
	} else if err != nil {
		return err
	}
	return nil
}

func populate(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) error {
	rng := rand.New(rand.NewSource(42))
	loc, _ := time.LoadLocation(Timezone)
	now := time.Now().In(loc)
	today := store.DateOf(now)

	// --- Plans ---------------------------------------------------------------
	type planSpec struct {
		name string
		days int
		rs   float64
		desc string
	}
	planSpecs := []planSpec{
		{"Monthly", 30, 1500, "Full gym access, billed monthly."},
		{"Quarterly", 90, 4000, "Full gym access for 3 months. Save 11%."},
		{"Half-yearly", 180, 7200, "Full gym access for 6 months. Save 20%."},
		{"Annual", 365, 12000, "Full gym access for 12 months. Best value."},
	}
	plans := make([]store.MembershipPlan, 0, len(planSpecs))
	for _, p := range planSpecs {
		pl, err := store.CreatePlan(ctx, tx, tenantID, store.PlanInput{Name: p.name, DurationDays: p.days, Price: p.rs, Description: p.desc, Active: true})
		if err != nil {
			return err
		}
		plans = append(plans, pl)
	}

	// --- Trainers ------------------------------------------------------------
	type trainerSpec struct {
		name  string
		phone string
		specs []string
	}
	trainerSpecs := []trainerSpec{
		{"Arjun Nair", "+91 98450 11001", []string{"Strength", "Powerlifting"}},
		{"Priya Menon", "+91 98450 11002", []string{"Weight loss", "HIIT", "Mobility"}},
		{"Vikram Shetty", "+91 98450 11003", []string{"Bodybuilding", "Nutrition"}},
		{"Divya Rao", "+91 98450 11004", []string{"Functional training", "Rehab"}},
	}
	trainers := make([]store.Trainer, 0, len(trainerSpecs))
	for i, t := range trainerSpecs {
		email := fmt.Sprintf("%s@coachrelin.fit", store.Slugify(t.name))
		phone := t.phone
		tr, err := store.CreateTrainer(ctx, tx, tenantID, store.TrainerInput{
			FullName: t.name, Email: &email, Phone: &phone, Specialties: t.specs, Active: i < 3 || rng.Intn(2) == 0,
		})
		if err != nil {
			return err
		}
		trainers = append(trainers, tr)
	}

	// --- Members -------------------------------------------------------------
	names := []string{
		"Aditya Kumar", "Sneha Iyer", "Rahul Verma", "Ananya Pillai", "Karthik Reddy", "Meera Krishnan",
		"Rohan Das", "Pooja Hegde", "Siddharth Jain", "Lakshmi Narayanan", "Nikhil Bhat", "Kavya Suresh",
		"Varun Mehta", "Ishita Sharma", "Manoj Gowda", "Riya Kapoor", "Suresh Babu", "Neha Joshi",
		"Abhishek Singh", "Tanvi Desai", "Harish Kumar", "Shruti Rao", "Deepak Patel", "Aishwarya Nair",
		"Ganesh Murthy", "Pallavi Shetty", "Vivek Anand", "Swati Mishra", "Rajesh Khanna", "Anjali Menon",
	}
	genders := []string{"male", "female", "male", "female", "male", "female"}
	members := make([]store.Member, 0, len(names))
	for i, n := range names {
		joined := today.AddDays(-(30 + rng.Intn(400)))
		dob := store.NewDate(1980+rng.Intn(25), time.Month(1+rng.Intn(12)), 1+rng.Intn(28))
		email := fmt.Sprintf("%s@example.com", store.Slugify(n))
		phone := fmt.Sprintf("+91 9%d%d %d%d%d%d%d", rng.Intn(10), rng.Intn(10), rng.Intn(10), rng.Intn(10), rng.Intn(10), rng.Intn(10), rng.Intn(10))
		gender := genders[i%len(genders)]
		status := "active"
		switch {
		case i%11 == 10:
			status = "inactive"
		case i%13 == 12:
			status = "frozen"
		}
		ec := "Family: +91 90000 0" + fmt.Sprintf("%04d", 1000+i)
		m, err := store.CreateMember(ctx, tx, tenantID, store.MemberInput{
			FullName: n, Email: &email, Phone: &phone, DateOfBirth: &dob, Gender: &gender,
			JoinedAt: joined, Status: status, EmergencyContact: &ec,
		})
		if err != nil {
			return err
		}
		members = append(members, m)
	}

	// --- Subscriptions & membership payments ---------------------------------
	methods := []string{"upi", "cash", "card", "bank_transfer", "upi", "upi"}
	for i, m := range members {
		plan := plans[rng.Intn(len(plans))]
		var start store.Date
		switch {
		case i%7 == 0: // expiring in the next week
			start = today.AddDays(-(plan.DurationDays - 1 - rng.Intn(7)))
		case i%7 == 1: // expired recently
			start = today.AddDays(-(plan.DurationDays + 5 + rng.Intn(40)))
		default: // active with time left
			start = today.AddDays(-rng.Intn(plan.DurationDays - 8))
		}
		if start.Before(m.JoinedAt.Time) {
			start = m.JoinedAt
		}
		end := start.AddDays(plan.DurationDays - 1)
		sub, err := store.CreateSubscription(ctx, tx, tenantID, m.ID, store.SubscriptionInput{
			PlanID: plan.ID, StartDate: start, EndDate: end, Price: plan.Price,
		})
		if err != nil {
			return err
		}
		// Most members paid in full; some paid partially; a few not at all.
		amount := plan.Price
		switch {
		case i%9 == 4:
			amount = plan.Price / 2
		case i%9 == 8:
			amount = 0
		}
		if amount > 0 {
			paidAt := time.Date(start.Year(), start.Month(), start.Day(), 10+rng.Intn(8), rng.Intn(60), 0, 0, loc)
			if _, err := store.CreatePayment(ctx, tx, tenantID, store.PaymentInput{
				MemberID: m.ID, SubscriptionID: &sub.ID, Amount: amount, Method: methods[rng.Intn(len(methods))], PaidAt: paidAt,
			}); err != nil {
				return err
			}
		}
		// Some members also have an older, expired subscription in history.
		if i%3 == 0 {
			oldPlan := plans[0]
			oldStart := start.AddDays(-oldPlan.DurationDays)
			if !oldStart.Before(m.JoinedAt.Time) {
				oldEnd := oldStart.AddDays(oldPlan.DurationDays - 1)
				old, err := store.CreateSubscription(ctx, tx, tenantID, m.ID, store.SubscriptionInput{
					PlanID: oldPlan.ID, StartDate: oldStart, EndDate: oldEnd, Price: oldPlan.Price,
				})
				if err != nil {
					return err
				}
				paidAt := time.Date(oldStart.Year(), oldStart.Month(), oldStart.Day(), 11, 0, 0, 0, loc)
				if _, err := store.CreatePayment(ctx, tx, tenantID, store.PaymentInput{
					MemberID: m.ID, SubscriptionID: &old.ID, Amount: oldPlan.Price, Method: "cash", PaidAt: paidAt,
				}); err != nil {
					return err
				}
			}
		}
	}

	// --- PT plans (reusable package templates) -------------------------------
	type ptPlanSpec struct {
		name     string
		sessions int
		days     int
		price    float64
		desc     string
	}
	ptPlanSpecs := []ptPlanSpec{
		{"4-session PT pack", 4, 60, 3600, "Four 1-hour sessions. A short block to try a trainer."},
		{"8-session PT pack", 8, 90, 6400, "Eight 1-hour sessions. Valid for 3 months."},
		{"12-session PT pack", 12, 90, 9000, "Twelve 1-hour sessions. The most popular pack."},
		{"20-session PT pack", 20, 120, 14000, "Twenty 1-hour sessions. Best value, valid for 4 months."},
	}
	ptPlans := make([]store.PTPlan, 0, len(ptPlanSpecs))
	for _, p := range ptPlanSpecs {
		days := p.days
		pl, err := store.CreatePTPlan(ctx, tx, tenantID, store.PTPlanInput{
			Name: p.name, Sessions: p.sessions, ValidityDays: &days, Price: p.price, Description: p.desc, Active: true,
		})
		if err != nil {
			return err
		}
		ptPlans = append(ptPlans, pl)
	}

	// --- PT packages & sessions ----------------------------------------------
	activeTrainers := make([]store.Trainer, 0)
	for _, t := range trainers {
		if t.Active {
			activeTrainers = append(activeTrainers, t)
		}
	}

	for i := 0; i < 12; i++ {
		m := members[(i*5)%len(members)]
		tr := activeTrainers[i%len(activeTrainers)]
		plan := ptPlans[1+rng.Intn(len(ptPlans)-1)] // skip the 4-session intro pack
		purchased := today.AddDays(-(7 + rng.Intn(60)))
		in := store.PackageInput{
			MemberID: m.ID, TrainerID: tr.ID, PurchasedAt: purchased, Status: "active",
		}
		in.ApplyPlan(plan, nil)
		pk, err := store.CreatePackage(ctx, tx, tenantID, in)
		if err != nil {
			return err
		}
		paidAt := time.Date(purchased.Year(), purchased.Month(), purchased.Day(), 12, 0, 0, 0, loc)
		payAmount := plan.Price
		if i%4 == 3 {
			payAmount = plan.Price * 0.5
		}
		if _, err := store.CreatePayment(ctx, tx, tenantID, store.PaymentInput{
			MemberID: m.ID, PTPackageID: &pk.ID, Amount: payAmount, Method: methods[rng.Intn(len(methods))], PaidAt: paidAt,
		}); err != nil {
			return err
		}

		// Past sessions: completed with the occasional no-show / cancellation.
		used := rng.Intn(plan.Sessions - 2)
		if i == 0 {
			used = plan.Sessions // one fully-used package -> auto-completed
		}
		day := purchased.AddDays(1)
		for k := 0; k < used; k++ {
			at := time.Date(day.Year(), day.Month(), day.Day(), 6+rng.Intn(13), []int{0, 30}[rng.Intn(2)], 0, 0, loc)
			status := "completed"
			if k%7 == 6 {
				status = "no_show"
			}
			if _, err := store.CreateSession(ctx, tx, tenantID, store.SessionInput{
				PackageID: pk.ID, ScheduledAt: at, DurationMinutes: 60, Status: status,
			}); err != nil {
				return err
			}
			day = day.AddDays(2 + rng.Intn(3))
			if !day.Before(today.Time) {
				break
			}
		}
		// Upcoming sessions for active packages: today + next few days.
		if used < plan.Sessions {
			remaining := plan.Sessions - used
			upcoming := 1 + rng.Intn(3)
			if upcoming > remaining {
				upcoming = remaining
			}
			for k := 0; k < upcoming; k++ {
				d := today.AddDays(k * (1 + rng.Intn(2)))
				at := time.Date(d.Year(), d.Month(), d.Day(), 7+rng.Intn(12), []int{0, 30}[rng.Intn(2)], 0, 0, loc)
				if _, err := store.CreateSession(ctx, tx, tenantID, store.SessionInput{
					PackageID: pk.ID, ScheduledAt: at, DurationMinutes: 60, Status: "scheduled",
				}); err != nil {
					if errors.Is(err, store.ErrNoSessionsLeft) || errors.Is(err, store.ErrPackageNotActive) {
						break
					}
					return err
				}
			}
		}
	}
	return nil
}
