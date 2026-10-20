package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/auth"
	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/config"
	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/db"
	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/store"
)

func newTestTokens() *auth.Tokens { return auth.NewTokens("integration-test-secret", time.Hour) }

// testEnv is a live API server backed by a real Postgres. Tests that use it
// are skipped unless TEST_DATABASE_URL is set, e.g.
//
//	TEST_DATABASE_URL=postgres://gym:gym@localhost:5432/gym?sslmode=disable go test ./...
type testEnv struct {
	t       *testing.T
	db      *db.DB
	srv     *httptest.Server
	tenants []uuid.UUID
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	ctx := context.Background()
	database, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := database.CheckRLSSafety(ctx); err != nil {
		t.Fatalf("test database role must not bypass RLS: %v", err)
	}
	if err := database.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	cfg := config.Config{JWTSecret: "integration-test-secret", SessionTTL: time.Hour}
	s := New(database, newTestTokens(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	srv := httptest.NewServer(s.Router())

	env := &testEnv{t: t, db: database, srv: srv}
	t.Cleanup(func() {
		srv.Close()
		for _, id := range env.tenants {
			if err := database.WithTx(ctx, func(tx pgx.Tx) error { return store.DeleteTenant(ctx, tx, id) }); err != nil {
				t.Errorf("cleanup tenant %s: %v", id, err)
			}
		}
		database.Close()
	})
	return env
}

type resp struct {
	status int
	body   []byte
	cookie *http.Cookie
}

func (r resp) json(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.body, v); err != nil {
		t.Fatalf("decode %q: %v", r.body, err)
	}
}

func (r resp) str(t *testing.T, key string) string {
	t.Helper()
	var m map[string]any
	r.json(t, &m)
	s, _ := m[key].(string)
	return s
}

// do performs a JSON request, optionally authenticated with a session cookie.
func (e *testEnv) do(method, path string, body any, cookie *http.Cookie) resp {
	e.t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			e.t.Fatalf("marshal: %v", err)
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, e.srv.URL+path, rd)
	if err != nil {
		e.t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	res, err := e.srv.Client().Do(req)
	if err != nil {
		e.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	out := resp{status: res.StatusCode}
	out.body, _ = io.ReadAll(res.Body)
	for _, c := range res.Cookies() {
		if c.Name == auth.CookieName {
			out.cookie = c
		}
	}
	return out
}

func (e *testEnv) want(r resp, status int, what string) {
	e.t.Helper()
	if r.status != status {
		e.t.Fatalf("%s: status = %d, want %d; body = %s", what, r.status, status, r.body)
	}
}

// register creates a tenant + owner and returns the session cookie and tenant id.
func (e *testEnv) register(gym string) (*http.Cookie, uuid.UUID, string) {
	e.t.Helper()
	email := fmt.Sprintf("owner-%s@%s.test", uuid.NewString()[:8], store.Slugify(gym))
	r := e.do(http.MethodPost, "/api/auth/register", map[string]any{
		"gym_name": gym, "full_name": "Owner", "email": email, "password": "password123",
		"timezone": "Asia/Kolkata", "currency": "INR",
	}, nil)
	e.want(r, http.StatusCreated, "register "+gym)
	if r.cookie == nil || r.cookie.Value == "" || !r.cookie.HttpOnly {
		e.t.Fatalf("register %s: expected httpOnly session cookie, got %+v", gym, r.cookie)
	}
	var out struct {
		Tenant store.Tenant `json:"tenant"`
	}
	r.json(e.t, &out)
	e.tenants = append(e.tenants, out.Tenant.ID)
	return r.cookie, out.Tenant.ID, email
}

func TestIntegration_AuthFlow(t *testing.T) {
	e := newTestEnv(t)
	cookie, tenantID, email := e.register("Auth Flow Gym")

	t.Run("me", func(t *testing.T) {
		r := e.do(http.MethodGet, "/api/auth/me", nil, cookie)
		e.want(r, http.StatusOK, "me")
		var out struct {
			User   store.User   `json:"user"`
			Tenant store.Tenant `json:"tenant"`
		}
		r.json(t, &out)
		if out.Tenant.ID != tenantID || out.Tenant.Name != "Auth Flow Gym" || !out.User.HasRole("owner") {
			t.Errorf("me = %+v", out)
		}
		if out.User.Email != email {
			t.Errorf("email = %q, want %q", out.User.Email, email)
		}
	})

	t.Run("duplicate email", func(t *testing.T) {
		r := e.do(http.MethodPost, "/api/auth/register", map[string]any{
			"gym_name": "Another", "full_name": "X", "email": email, "password": "password123",
		}, nil)
		e.want(r, http.StatusBadRequest, "duplicate register")
		var out struct {
			Fields map[string]string `json:"fields"`
		}
		r.json(t, &out)
		if out.Fields["email"] != "is already registered" {
			t.Errorf("fields = %v", out.Fields)
		}
	})

	t.Run("validation", func(t *testing.T) {
		r := e.do(http.MethodPost, "/api/auth/register", map[string]any{
			"gym_name": "", "email": "nope", "password": "short", "timezone": "Mars/Olympus", "currency": "RUPEES",
		}, nil)
		e.want(r, http.StatusBadRequest, "invalid register")
		var out struct {
			Fields map[string]string `json:"fields"`
		}
		r.json(t, &out)
		for _, f := range []string{"gym_name", "full_name", "email", "password", "timezone", "currency"} {
			if out.Fields[f] == "" {
				t.Errorf("expected a validation message for %s; got %v", f, out.Fields)
			}
		}
	})

	t.Run("login", func(t *testing.T) {
		r := e.do(http.MethodPost, "/api/auth/login", map[string]string{"email": email, "password": "wrong-password"}, nil)
		e.want(r, http.StatusUnauthorized, "bad login")

		r = e.do(http.MethodPost, "/api/auth/login", map[string]string{"email": "nobody@nowhere.test", "password": "password123"}, nil)
		e.want(r, http.StatusUnauthorized, "unknown user login")

		// Email matching is case-insensitive.
		r = e.do(http.MethodPost, "/api/auth/login", map[string]string{"email": "  " + upperFirst(email), "password": "password123"}, nil)
		e.want(r, http.StatusOK, "good login")
		if r.cookie == nil || r.cookie.Value == "" {
			t.Fatal("login did not set a session cookie")
		}
	})

	t.Run("logout clears cookie", func(t *testing.T) {
		r := e.do(http.MethodPost, "/api/auth/logout", nil, cookie)
		e.want(r, http.StatusNoContent, "logout")
		if r.cookie == nil || r.cookie.MaxAge >= 0 {
			t.Errorf("logout cookie = %+v, want expired", r.cookie)
		}
	})

	t.Run("forged token for deleted tenant", func(t *testing.T) {
		tok, _ := newTestTokens().Issue(uuid.New(), uuid.New(), []string{"owner"}, uuid.Nil)
		r := e.do(http.MethodGet, "/api/members", nil, &http.Cookie{Name: auth.CookieName, Value: tok})
		e.want(r, http.StatusUnauthorized, "token for unknown tenant")
	})
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	b := []byte(s)
	if b[0] >= 'a' && b[0] <= 'z' {
		b[0] -= 'a' - 'A'
	}
	return string(b)
}

func TestIntegration_MembershipAndPTFlow(t *testing.T) {
	e := newTestEnv(t)
	cookie, _, _ := e.register("Flow Gym")
	today := time.Now().Format("2006-01-02")

	// Plan
	r := e.do(http.MethodPost, "/api/plans", map[string]any{"name": "Monthly", "duration_days": 30, "price": 1500}, cookie)
	e.want(r, http.StatusCreated, "create plan")
	var plan store.MembershipPlan
	r.json(t, &plan)

	// Trainer
	r = e.do(http.MethodPost, "/api/trainers", map[string]any{"full_name": "Coach T", "specialties": []string{"Strength"}}, cookie)
	e.want(r, http.StatusCreated, "create trainer")
	var trainer store.Trainer
	r.json(t, &trainer)

	// Member
	r = e.do(http.MethodPost, "/api/members", map[string]any{"full_name": "Test Member", "phone": "+91 90000 00000", "joined_at": today}, cookie)
	e.want(r, http.StatusCreated, "create member")
	var member store.Member
	r.json(t, &member)
	if member.MembershipStatus != "none" {
		t.Errorf("new member membership_status = %q, want none", member.MembershipStatus)
	}

	// Validation on member
	r = e.do(http.MethodPost, "/api/members", map[string]any{"full_name": "", "status": "bogus"}, cookie)
	e.want(r, http.StatusBadRequest, "invalid member")

	// Subscription: end date defaults to start + duration - 1
	r = e.do(http.MethodPost, "/api/members/"+member.ID.String()+"/subscriptions",
		map[string]any{"plan_id": plan.ID.String(), "start_date": today}, cookie)
	e.want(r, http.StatusCreated, "create subscription")
	var sub store.Subscription
	r.json(t, &sub)
	if got := sub.EndDate.String(); got != sub.StartDate.AddDays(29).String() {
		t.Errorf("end_date = %s, want start+29 = %s", got, sub.StartDate.AddDays(29))
	}
	if sub.Status != "active" {
		t.Errorf("subscription status = %q, want active", sub.Status)
	}

	// Member now active with full dues
	r = e.do(http.MethodGet, "/api/members/"+member.ID.String(), nil, cookie)
	e.want(r, http.StatusOK, "get member")
	var detail memberDetail
	r.json(t, &detail)
	if detail.MembershipStatus != "active" || detail.OutstandingDues != 1500 || detail.CurrentPlanName == nil || *detail.CurrentPlanName != "Monthly" {
		t.Errorf("after subscription: status=%s dues=%v plan=%v", detail.MembershipStatus, detail.OutstandingDues, detail.CurrentPlanName)
	}

	// Partial payment reduces dues
	r = e.do(http.MethodPost, "/api/payments", map[string]any{
		"member_id": member.ID.String(), "subscription_id": sub.ID.String(), "amount": 500, "method": "upi",
	}, cookie)
	e.want(r, http.StatusCreated, "create payment")

	r = e.do(http.MethodGet, "/api/members/"+member.ID.String(), nil, cookie)
	e.want(r, http.StatusOK, "get member after payment")
	r.json(t, &detail)
	if detail.OutstandingDues != 1000 {
		t.Errorf("dues after 500 payment = %v, want 1000", detail.OutstandingDues)
	}
	if len(detail.Payments) != 1 || len(detail.Subscriptions) != 1 {
		t.Errorf("detail has %d payments, %d subscriptions", len(detail.Payments), len(detail.Subscriptions))
	}

	// Payment linked to both a subscription and a package is rejected
	r = e.do(http.MethodPost, "/api/payments", map[string]any{
		"member_id": member.ID.String(), "subscription_id": sub.ID.String(), "pt_package_id": uuid.NewString(), "amount": 1,
	}, cookie)
	e.want(r, http.StatusBadRequest, "payment with sub and package")

	// PT package with 2 sessions
	r = e.do(http.MethodPost, "/api/pt/packages", map[string]any{
		"member_id": member.ID.String(), "trainer_id": trainer.ID.String(), "name": "Starter",
		"total_sessions": 2, "price": 2000, "purchased_at": today,
	}, cookie)
	e.want(r, http.StatusCreated, "create package")
	var pkg store.PTPackage
	r.json(t, &pkg)
	if pkg.Status != "active" || pkg.SessionsRemaining != 2 {
		t.Errorf("package = status %s remaining %d", pkg.Status, pkg.SessionsRemaining)
	}

	// Schedule two sessions, third is rejected
	base := time.Now().Add(24 * time.Hour).Truncate(time.Hour)
	var sessions []store.PTSession
	for i := 0; i < 2; i++ {
		r = e.do(http.MethodPost, "/api/pt/sessions", map[string]any{
			"package_id": pkg.ID.String(), "scheduled_at": base.Add(time.Duration(i) * time.Hour).Format(time.RFC3339), "duration_minutes": 60,
		}, cookie)
		e.want(r, http.StatusCreated, fmt.Sprintf("create session %d", i+1))
		var s store.PTSession
		r.json(t, &s)
		if s.TrainerID != trainer.ID {
			t.Errorf("session trainer = %s, want package trainer %s", s.TrainerID, trainer.ID)
		}
		sessions = append(sessions, s)
	}
	r = e.do(http.MethodPost, "/api/pt/sessions", map[string]any{
		"package_id": pkg.ID.String(), "scheduled_at": base.Add(3 * time.Hour).Format(time.RFC3339),
	}, cookie)
	e.want(r, http.StatusConflict, "third session on a 2-session package")

	// Complete one, no-show the other -> package completed
	r = e.do(http.MethodPatch, "/api/pt/sessions/"+sessions[0].ID.String(), map[string]any{"status": "completed"}, cookie)
	e.want(r, http.StatusOK, "complete session")
	r = e.do(http.MethodPatch, "/api/pt/sessions/"+sessions[1].ID.String(), map[string]any{"status": "no_show"}, cookie)
	e.want(r, http.StatusOK, "no-show session")

	r = e.do(http.MethodGet, "/api/pt/packages/"+pkg.ID.String(), nil, cookie)
	e.want(r, http.StatusOK, "get package")
	r.json(t, &pkg)
	if pkg.Status != "completed" || pkg.SessionsUsed != 2 || pkg.SessionsRemaining != 0 {
		t.Errorf("package after sessions = status %s used %d remaining %d", pkg.Status, pkg.SessionsUsed, pkg.SessionsRemaining)
	}

	// Scheduling against a completed package is rejected
	r = e.do(http.MethodPost, "/api/pt/sessions", map[string]any{
		"package_id": pkg.ID.String(), "scheduled_at": base.Add(4 * time.Hour).Format(time.RFC3339),
	}, cookie)
	e.want(r, http.StatusConflict, "session on completed package")

	// Reverting a session re-activates the package
	r = e.do(http.MethodPatch, "/api/pt/sessions/"+sessions[1].ID.String(), map[string]any{"status": "scheduled"}, cookie)
	e.want(r, http.StatusOK, "revert session")
	r = e.do(http.MethodGet, "/api/pt/packages/"+pkg.ID.String(), nil, cookie)
	e.want(r, http.StatusOK, "get package again")
	r.json(t, &pkg)
	if pkg.Status != "active" || pkg.SessionsUsed != 1 {
		t.Errorf("package after revert = status %s used %d", pkg.Status, pkg.SessionsUsed)
	}

	// Trainer with packages cannot be deleted (RESTRICT)
	r = e.do(http.MethodDelete, "/api/trainers/"+trainer.ID.String(), nil, cookie)
	e.want(r, http.StatusConflict, "delete trainer in use")

	// Plan with subscriptions cannot be deleted
	r = e.do(http.MethodDelete, "/api/plans/"+plan.ID.String(), nil, cookie)
	e.want(r, http.StatusConflict, "delete plan in use")

	// Dashboard reflects the data
	r = e.do(http.MethodGet, "/api/dashboard", nil, cookie)
	e.want(r, http.StatusOK, "dashboard")
	var dash store.Dashboard
	r.json(t, &dash)
	if dash.TotalMembers != 1 || dash.ActiveMembers != 1 || dash.ActiveTrainers != 1 {
		t.Errorf("dashboard = members %d active %d trainers %d", dash.TotalMembers, dash.ActiveMembers, dash.ActiveTrainers)
	}
	if dash.OutstandingDues != 1000+2000 {
		t.Errorf("dashboard dues = %v, want 3000 (1000 membership + 2000 unpaid package)", dash.OutstandingDues)
	}

	// Deleting the member cascades sessions/packages/payments
	r = e.do(http.MethodDelete, "/api/members/"+member.ID.String(), nil, cookie)
	e.want(r, http.StatusNoContent, "delete member")
	r = e.do(http.MethodGet, "/api/pt/packages/"+pkg.ID.String(), nil, cookie)
	e.want(r, http.StatusNotFound, "package gone after member delete")
}

// Buying a PT package is like adding a membership: pick a reusable plan
// (sessions, validity, price) and optionally customise any of those values.
func TestIntegration_PTPlans(t *testing.T) {
	e := newTestEnv(t)
	cookie, _, _ := e.register("PT Plan Gym")
	today := time.Now().Format("2006-01-02")

	r := e.do(http.MethodPost, "/api/pt/plans", map[string]any{
		"name": "", "sessions": 0, "price": -1, "validity_days": 0,
	}, cookie)
	e.want(r, http.StatusBadRequest, "invalid pt plan")

	r = e.do(http.MethodPost, "/api/pt/plans", map[string]any{
		"name": "8-session pack", "sessions": 8, "validity_days": 90, "price": 6400, "description": "Eight hours",
	}, cookie)
	e.want(r, http.StatusCreated, "create pt plan")
	var plan store.PTPlan
	r.json(t, &plan)
	if plan.Sessions != 8 || plan.Price != 6400 || plan.ValidityDays == nil || *plan.ValidityDays != 90 || !plan.Active {
		t.Errorf("plan = %+v", plan)
	}

	r = e.do(http.MethodPost, "/api/trainers", map[string]any{"full_name": "Coach"}, cookie)
	e.want(r, http.StatusCreated, "create trainer")
	var trainer store.Trainer
	r.json(t, &trainer)
	r = e.do(http.MethodPost, "/api/members", map[string]any{"full_name": "PT Member", "joined_at": today}, cookie)
	e.want(r, http.StatusCreated, "create member")
	var member store.Member
	r.json(t, &member)

	// The simple form: member + trainer + plan. Name, sessions, price, expiry
	// come from the plan.
	r = e.do(http.MethodPost, "/api/pt/packages", map[string]any{
		"member_id": member.ID.String(), "trainer_id": trainer.ID.String(), "pt_plan_id": plan.ID.String(), "purchased_at": today,
	}, cookie)
	e.want(r, http.StatusCreated, "buy from plan")
	var pkg store.PTPackage
	r.json(t, &pkg)
	if pkg.Name != "8-session pack" || pkg.TotalSessions != 8 || pkg.Price != 6400 {
		t.Errorf("package from plan = name %q sessions %d price %v", pkg.Name, pkg.TotalSessions, pkg.Price)
	}
	if pkg.PTPlanID == nil || *pkg.PTPlanID != plan.ID {
		t.Errorf("pt_plan_id = %v, want %s", pkg.PTPlanID, plan.ID)
	}
	purchased, err := store.ParseDate(today)
	if err != nil {
		t.Fatal(err)
	}
	wantExp := purchased.AddDays(90).String()
	if pkg.ExpiresAt == nil || pkg.ExpiresAt.String() != wantExp {
		t.Errorf("expires_at = %v, want %s", pkg.ExpiresAt, wantExp)
	}

	// Customise sessions, price and drop the expiry.
	r = e.do(http.MethodPost, "/api/pt/packages", map[string]any{
		"member_id": member.ID.String(), "trainer_id": trainer.ID.String(), "pt_plan_id": plan.ID.String(),
		"total_sessions": 10, "price": 7000, "expires_at": "", "purchased_at": today,
	}, cookie)
	e.want(r, http.StatusCreated, "customise plan")
	var customised store.PTPackage
	r.json(t, &customised)
	if customised.TotalSessions != 10 || customised.Price != 7000 || customised.Name != "8-session pack" || customised.ExpiresAt != nil {
		t.Errorf("customised = sessions %d price %v name %q exp %v", customised.TotalSessions, customised.Price, customised.Name, customised.ExpiresAt)
	}

	// Custom package with no plan still works.
	r = e.do(http.MethodPost, "/api/pt/packages", map[string]any{
		"member_id": member.ID.String(), "trainer_id": trainer.ID.String(),
		"name": "One-off", "total_sessions": 3, "price": 2500, "purchased_at": today,
	}, cookie)
	e.want(r, http.StatusCreated, "custom package")
	var custom store.PTPackage
	r.json(t, &custom)
	if custom.PTPlanID != nil || custom.Name != "One-off" || custom.TotalSessions != 3 {
		t.Errorf("custom package = %+v", custom)
	}

	// Inactive plans cannot be sold, but can still be edited.
	r = e.do(http.MethodPut, "/api/pt/plans/"+plan.ID.String(), map[string]any{
		"name": plan.Name, "sessions": plan.Sessions, "validity_days": plan.ValidityDays, "price": plan.Price, "active": false,
	}, cookie)
	e.want(r, http.StatusOK, "deactivate plan")
	r = e.do(http.MethodPost, "/api/pt/packages", map[string]any{
		"member_id": member.ID.String(), "trainer_id": trainer.ID.String(), "pt_plan_id": plan.ID.String(),
	}, cookie)
	e.want(r, http.StatusBadRequest, "buy inactive plan")

	r = e.do(http.MethodGet, "/api/pt/plans?active=true", nil, cookie)
	e.want(r, http.StatusOK, "list active pt plans")
	var listed struct {
		Items []store.PTPlan `json:"items"`
	}
	r.json(t, &listed)
	if len(listed.Items) != 0 {
		t.Errorf("active list = %d, want 0 after deactivating the only plan", len(listed.Items))
	}

	r = e.do(http.MethodDelete, "/api/pt/plans/"+plan.ID.String(), nil, cookie)
	e.want(r, http.StatusConflict, "delete plan in use")
}

// Unlinked ("General") payments must still reduce what a member owes. They
// are applied to the member's billable items oldest first, and both the
// member-level dues and the per-item "paid" figures reflect that.
func TestIntegration_UnlinkedPaymentsReduceDues(t *testing.T) {
	e := newTestEnv(t)
	cookie, _, _ := e.register("Credit Gym")
	today := time.Now().Format("2006-01-02")

	r := e.do(http.MethodPost, "/api/plans", map[string]any{"name": "Monthly", "duration_days": 30, "price": 1500}, cookie)
	e.want(r, http.StatusCreated, "create plan")
	var plan store.MembershipPlan
	r.json(t, &plan)

	r = e.do(http.MethodPost, "/api/trainers", map[string]any{"full_name": "Coach"}, cookie)
	e.want(r, http.StatusCreated, "create trainer")
	var trainer store.Trainer
	r.json(t, &trainer)

	r = e.do(http.MethodPost, "/api/members", map[string]any{"full_name": "Credit Member", "joined_at": today}, cookie)
	e.want(r, http.StatusCreated, "create member")
	var member store.Member
	r.json(t, &member)

	getDetail := func() memberDetail {
		t.Helper()
		r := e.do(http.MethodGet, "/api/members/"+member.ID.String(), nil, cookie)
		e.want(r, http.StatusOK, "get member")
		var d memberDetail
		r.json(t, &d)
		return d
	}
	dashboardDues := func() float64 {
		t.Helper()
		r := e.do(http.MethodGet, "/api/dashboard", nil, cookie)
		e.want(r, http.StatusOK, "dashboard")
		var d store.Dashboard
		r.json(t, &d)
		return d.OutstandingDues
	}

	// Older item: a 1500 subscription. Newer item: a 2000 PT package.
	r = e.do(http.MethodPost, "/api/members/"+member.ID.String()+"/subscriptions", map[string]any{"plan_id": plan.ID.String(), "start_date": today}, cookie)
	e.want(r, http.StatusCreated, "create subscription")
	var sub store.Subscription
	r.json(t, &sub)
	r = e.do(http.MethodPost, "/api/pt/packages", map[string]any{
		"member_id": member.ID.String(), "trainer_id": trainer.ID.String(), "name": "Pack", "total_sessions": 4, "price": 2000, "purchased_at": today,
	}, cookie)
	e.want(r, http.StatusCreated, "create package")
	var pkg store.PTPackage
	r.json(t, &pkg)

	if d := getDetail(); d.OutstandingDues != 3500 {
		t.Fatalf("dues before any payment = %v, want 3500", d.OutstandingDues)
	}

	// A general payment of 1500 (exactly the membership) with no reference.
	r = e.do(http.MethodPost, "/api/payments", map[string]any{"member_id": member.ID.String(), "amount": 1500, "method": "upi"}, cookie)
	e.want(r, http.StatusCreated, "general payment")

	d := getDetail()
	if d.OutstandingDues != 2000 {
		t.Errorf("dues after 1500 general payment = %v, want 2000", d.OutstandingDues)
	}
	if got := dashboardDues(); got != 2000 {
		t.Errorf("dashboard dues = %v, want 2000", got)
	}
	// FIFO: the older subscription is shown as fully paid, the package untouched.
	if len(d.Subscriptions) != 1 || d.Subscriptions[0].Paid != 1500 {
		t.Errorf("subscription paid = %+v, want 1500", d.Subscriptions)
	}
	if len(d.Packages) != 1 || d.Packages[0].Paid != 0 {
		t.Errorf("package paid = %+v, want 0", d.Packages)
	}

	// Another general payment of 500 spills over to the package.
	r = e.do(http.MethodPost, "/api/payments", map[string]any{"member_id": member.ID.String(), "amount": 500, "method": "cash"}, cookie)
	e.want(r, http.StatusCreated, "second general payment")
	d = getDetail()
	if d.OutstandingDues != 1500 || d.Packages[0].Paid != 500 || d.Subscriptions[0].Paid != 1500 {
		t.Errorf("after 500 more: dues=%v sub.paid=%v pkg.paid=%v; want 1500/1500/500",
			d.OutstandingDues, d.Subscriptions[0].Paid, d.Packages[0].Paid)
	}
	r = e.do(http.MethodGet, "/api/pt/packages/"+pkg.ID.String(), nil, cookie)
	e.want(r, http.StatusOK, "get package")
	r.json(t, &pkg)
	if pkg.Paid != 500 {
		t.Errorf("package endpoint paid = %v, want 500", pkg.Paid)
	}

	// A linked payment on the package combines with the allocated credit.
	r = e.do(http.MethodPost, "/api/payments", map[string]any{
		"member_id": member.ID.String(), "pt_package_id": pkg.ID.String(), "amount": 1000, "method": "card",
	}, cookie)
	e.want(r, http.StatusCreated, "linked package payment")
	d = getDetail()
	if d.OutstandingDues != 500 || d.Packages[0].Paid != 1500 {
		t.Errorf("after linked 1000: dues=%v pkg.paid=%v; want 500/1500", d.OutstandingDues, d.Packages[0].Paid)
	}

	// Overpaying in general never produces negative dues or paid > price.
	r = e.do(http.MethodPost, "/api/payments", map[string]any{"member_id": member.ID.String(), "amount": 5000, "method": "cash"}, cookie)
	e.want(r, http.StatusCreated, "overpayment")
	d = getDetail()
	if d.OutstandingDues != 0 || d.Packages[0].Paid != 2000 || d.Subscriptions[0].Paid != 1500 {
		t.Errorf("after overpayment: dues=%v sub.paid=%v pkg.paid=%v; want 0/1500/2000",
			d.OutstandingDues, d.Subscriptions[0].Paid, d.Packages[0].Paid)
	}
	if got := dashboardDues(); got != 0 {
		t.Errorf("dashboard dues after overpayment = %v, want 0", got)
	}

	// Members list reports the same figure as the detail endpoint.
	r = e.do(http.MethodGet, "/api/members", nil, cookie)
	e.want(r, http.StatusOK, "list members")
	var page store.Page[store.Member]
	r.json(t, &page)
	if len(page.Items) != 1 || page.Items[0].OutstandingDues != 0 {
		t.Errorf("members list dues = %+v, want 0", page.Items)
	}
}

func TestIntegration_TenantIsolation(t *testing.T) {
	e := newTestEnv(t)
	cookieA, tenantA, _ := e.register("Gym A")
	cookieB, tenantB, _ := e.register("Gym B")
	if tenantA == tenantB {
		t.Fatal("tenants must differ")
	}
	today := time.Now().Format("2006-01-02")

	// Seed tenant A with a member, trainer, plan and package.
	r := e.do(http.MethodPost, "/api/members", map[string]any{"full_name": "Alice A", "joined_at": today}, cookieA)
	e.want(r, http.StatusCreated, "A create member")
	var memberA store.Member
	r.json(t, &memberA)

	r = e.do(http.MethodPost, "/api/trainers", map[string]any{"full_name": "Trainer A"}, cookieA)
	e.want(r, http.StatusCreated, "A create trainer")
	var trainerA store.Trainer
	r.json(t, &trainerA)

	r = e.do(http.MethodPost, "/api/plans", map[string]any{"name": "Plan A", "duration_days": 30, "price": 100}, cookieA)
	e.want(r, http.StatusCreated, "A create plan")
	var planA store.MembershipPlan
	r.json(t, &planA)

	r = e.do(http.MethodPost, "/api/pt/plans", map[string]any{"name": "Pack A", "sessions": 5, "price": 500}, cookieA)
	e.want(r, http.StatusCreated, "A create pt plan")
	var ptPlanA store.PTPlan
	r.json(t, &ptPlanA)

	r = e.do(http.MethodPost, "/api/pt/packages", map[string]any{
		"member_id": memberA.ID.String(), "trainer_id": trainerA.ID.String(), "pt_plan_id": ptPlanA.ID.String(), "purchased_at": today,
	}, cookieA)
	e.want(r, http.StatusCreated, "A create package")
	var pkgA store.PTPackage
	r.json(t, &pkgA)

	// B sees none of it.
	for _, path := range []string{"/api/members", "/api/trainers", "/api/plans", "/api/pt/plans", "/api/pt/packages", "/api/pt/sessions", "/api/payments"} {
		r = e.do(http.MethodGet, path, nil, cookieB)
		e.want(r, http.StatusOK, "B list "+path)
		var page struct {
			Items []json.RawMessage `json:"items"`
			Total int               `json:"total"`
		}
		r.json(t, &page)
		if page.Total != 0 || len(page.Items) != 0 {
			t.Errorf("B %s sees %d rows from tenant A", path, page.Total)
		}
	}
	r = e.do(http.MethodGet, "/api/plans", nil, cookieA)
	e.want(r, http.StatusOK, "A list plans")

	// B cannot read A's rows by id.
	for _, path := range []string{
		"/api/members/" + memberA.ID.String(),
		"/api/trainers/" + trainerA.ID.String(),
		"/api/pt/packages/" + pkgA.ID.String(),
	} {
		r = e.do(http.MethodGet, path, nil, cookieB)
		e.want(r, http.StatusNotFound, "B get "+path)
	}

	// B cannot modify or delete A's rows.
	r = e.do(http.MethodPut, "/api/members/"+memberA.ID.String(), map[string]any{"full_name": "Hijacked", "joined_at": today}, cookieB)
	e.want(r, http.StatusNotFound, "B update A member")
	r = e.do(http.MethodDelete, "/api/members/"+memberA.ID.String(), nil, cookieB)
	e.want(r, http.StatusNotFound, "B delete A member")
	r = e.do(http.MethodDelete, "/api/pt/packages/"+pkgA.ID.String(), nil, cookieB)
	e.want(r, http.StatusNotFound, "B delete A package")

	// B cannot attach its own data to A's rows.
	r = e.do(http.MethodPost, "/api/payments", map[string]any{"member_id": memberA.ID.String(), "amount": 10}, cookieB)
	e.want(r, http.StatusBadRequest, "B pay for A member")
	r = e.do(http.MethodPost, "/api/members/"+memberA.ID.String()+"/subscriptions", map[string]any{"plan_id": planA.ID.String(), "start_date": today}, cookieB)
	if r.status != http.StatusNotFound && r.status != http.StatusBadRequest {
		t.Fatalf("B subscribe A member = %d, want 404/400; body %s", r.status, r.body)
	}
	r = e.do(http.MethodPost, "/api/pt/sessions", map[string]any{"package_id": pkgA.ID.String(), "scheduled_at": time.Now().Add(time.Hour).Format(time.RFC3339)}, cookieB)
	e.want(r, http.StatusBadRequest, "B schedule on A package")

	// B creating a member with A's plan/trainer ids fails rather than linking across tenants.
	r = e.do(http.MethodPost, "/api/members", map[string]any{"full_name": "Bob B", "joined_at": today}, cookieB)
	e.want(r, http.StatusCreated, "B create member")
	var memberB store.Member
	r.json(t, &memberB)
	r = e.do(http.MethodPost, "/api/pt/packages", map[string]any{
		"member_id": memberB.ID.String(), "trainer_id": trainerA.ID.String(), "name": "Cross", "total_sessions": 1, "price": 1, "purchased_at": today,
	}, cookieB)
	if r.status < 400 || r.status >= 500 {
		t.Fatalf("B package with A trainer = %d, want 4xx; body %s", r.status, r.body)
	}

	// A's data is intact.
	r = e.do(http.MethodGet, "/api/members/"+memberA.ID.String(), nil, cookieA)
	e.want(r, http.StatusOK, "A get member")
	var detail memberDetail
	r.json(t, &detail)
	if detail.FullName != "Alice A" || len(detail.Packages) != 1 {
		t.Errorf("A member = %s with %d packages", detail.FullName, len(detail.Packages))
	}

	// Settings are per tenant.
	r = e.do(http.MethodPatch, "/api/settings/tenant", map[string]any{"name": "Gym B Renamed", "currency": "USD"}, cookieB)
	e.want(r, http.StatusOK, "B patch settings")
	r = e.do(http.MethodGet, "/api/settings/tenant", nil, cookieA)
	e.want(r, http.StatusOK, "A get settings")
	var tenant store.Tenant
	r.json(t, &tenant)
	if tenant.Name != "Gym A" || tenant.Currency != "INR" {
		t.Errorf("A settings changed by B: %+v", tenant)
	}

	r = e.do(http.MethodPatch, "/api/settings/tenant", map[string]any{"gstin": "nope"}, cookieA)
	e.want(r, http.StatusBadRequest, "invalid GSTIN")
	r = e.do(http.MethodPatch, "/api/settings/tenant", map[string]any{"gstin": "32ABCDE1234F1Z5"}, cookieA)
	e.want(r, http.StatusOK, "A set GSTIN")
	r.json(t, &tenant)
	if tenant.Gstin == nil || *tenant.Gstin != "32ABCDE1234F1Z5" {
		t.Errorf("GSTIN = %v", tenant.Gstin)
	}

	r = e.do(http.MethodPatch, "/api/settings/tenant", map[string]any{"gst_percent": 101}, cookieA)
	e.want(r, http.StatusBadRequest, "invalid GST percent")
	r = e.do(http.MethodPatch, "/api/settings/tenant", map[string]any{"gst_percent": 18}, cookieA)
	e.want(r, http.StatusOK, "A set GST percent")
	r.json(t, &tenant)
	if tenant.GstPercent != 18 {
		t.Errorf("GST percent = %v", tenant.GstPercent)
	}

	// A token from tenant B cannot be used to act as A even with a valid user id.
	// (Tenant comes from the signed token, never from the request.)
	r = e.do(http.MethodGet, "/api/members?tenant_id="+tenantA.String(), nil, cookieB)
	e.want(r, http.StatusOK, "B list with tenant_id param")
	var page struct {
		Total int `json:"total"`
	}
	r.json(t, &page)
	if page.Total != 1 {
		t.Errorf("B sees %d members, want only its own 1", page.Total)
	}
}

func TestIntegration_Roles(t *testing.T) {
	e := newTestEnv(t)
	ownerCookie, _, _ := e.register("Roles Gym")
	today := store.DateOf(time.Now()).String()

	r := e.do(http.MethodPost, "/api/trainers", map[string]any{"full_name": "Coach Role"}, ownerCookie)
	e.want(r, http.StatusCreated, "trainer")
	var trainer store.Trainer
	r.json(t, &trainer)

	r = e.do(http.MethodPost, "/api/users", map[string]any{
		"full_name": "Front Desk", "email": fmt.Sprintf("desk-%s@roles.test", uuid.NewString()[:8]),
		"password": "password123", "role": "cashier",
	}, ownerCookie)
	e.want(r, http.StatusCreated, "create receptionist from legacy cashier")
	var desk store.User
	r.json(t, &desk)
	if !desk.HasRole(store.RoleReceptionist) || desk.HasRole("cashier") {
		t.Fatalf("receptionist roles = %v", desk.Roles)
	}

	r = e.do(http.MethodPost, "/api/auth/login", map[string]string{"email": desk.Email, "password": "password123"}, nil)
	e.want(r, http.StatusOK, "receptionist login")
	deskCookie := r.cookie

	r = e.do(http.MethodPost, "/api/members", map[string]any{"full_name": "Walk-in", "joined_at": today}, deskCookie)
	e.want(r, http.StatusCreated, "receptionist create member")
	var member store.Member
	r.json(t, &member)

	r = e.do(http.MethodPost, "/api/plans", map[string]any{"name": "Monthly", "duration_days": 30, "price": 1000}, deskCookie)
	e.want(r, http.StatusForbidden, "receptionist cannot create plan")

	r = e.do(http.MethodPatch, "/api/settings/tenant", map[string]any{"name": "Hacked"}, deskCookie)
	e.want(r, http.StatusForbidden, "receptionist cannot change settings")

	r = e.do(http.MethodDelete, "/api/members/"+member.ID.String(), nil, deskCookie)
	e.want(r, http.StatusForbidden, "receptionist cannot delete member")

	r = e.do(http.MethodGet, "/api/users", nil, deskCookie)
	e.want(r, http.StatusForbidden, "receptionist cannot list users")

	trainerEmail := fmt.Sprintf("tr-%s@roles.test", uuid.NewString()[:8])
	r = e.do(http.MethodPost, "/api/users", map[string]any{
		"full_name": "Coach Role", "email": trainerEmail, "password": "password123",
		"role": "trainer", "trainer_id": trainer.ID.String(),
	}, ownerCookie)
	e.want(r, http.StatusCreated, "create trainer login")

	r = e.do(http.MethodPost, "/api/users", map[string]any{
		"full_name": "Coach Role 2", "email": fmt.Sprintf("tr2-%s@roles.test", uuid.NewString()[:8]),
		"password": "password123", "role": "trainer", "trainer_id": trainer.ID.String(),
	}, ownerCookie)
	e.want(r, http.StatusBadRequest, "second login for the same trainer")

	r = e.do(http.MethodPost, "/api/auth/login", map[string]string{"email": trainerEmail, "password": "password123"}, nil)
	e.want(r, http.StatusOK, "trainer login")
	trCookie := r.cookie

	r = e.do(http.MethodPost, "/api/members", map[string]any{"full_name": "Nope", "joined_at": today}, trCookie)
	e.want(r, http.StatusForbidden, "trainer cannot create member")

	r = e.do(http.MethodGet, "/api/payments", nil, trCookie)
	e.want(r, http.StatusForbidden, "trainer cannot list payments")

	r = e.do(http.MethodGet, "/api/plans", nil, trCookie)
	e.want(r, http.StatusForbidden, "trainer cannot list plans")

	r = e.do(http.MethodPost, "/api/pt/packages", map[string]any{
		"member_id": member.ID.String(), "trainer_id": trainer.ID.String(),
		"name": "8-pack", "total_sessions": 8, "price": 4000, "purchased_at": today,
	}, ownerCookie)
	e.want(r, http.StatusCreated, "owner sells package")

	r = e.do(http.MethodGet, "/api/members", nil, trCookie)
	e.want(r, http.StatusOK, "trainer list assigned members")
	var page store.Page[store.Member]
	r.json(t, &page)
	if page.Total != 1 || page.Items[0].ID != member.ID {
		t.Fatalf("trainer members = %+v, want the assigned walk-in", page)
	}

	r = e.do(http.MethodGet, "/api/dashboard", nil, trCookie)
	e.want(r, http.StatusOK, "trainer dashboard")
	var dash store.Dashboard
	r.json(t, &dash)
	if dash.RevenueThisMonth != 0 || len(dash.RecentPayments) != 0 {
		t.Errorf("trainer dashboard leaked finance: %+v", dash)
	}
	if dash.ActivePTPackages != 1 {
		t.Errorf("trainer active packages = %d, want 1", dash.ActivePTPackages)
	}

	r = e.do(http.MethodPost, "/api/trainers", map[string]any{"full_name": "Desk Coach"}, ownerCookie)
	e.want(r, http.StatusCreated, "second trainer")
	var deskTrainer store.Trainer
	r.json(t, &deskTrainer)

	r = e.do(http.MethodPut, "/api/users/"+desk.ID.String(), map[string]any{
		"roles": []string{"receptionist", "trainer"}, "trainer_id": deskTrainer.ID.String(),
	}, ownerCookie)
	e.want(r, http.StatusOK, "receptionist also becomes trainer")
	r.json(t, &desk)
	if !desk.HasRole(store.RoleReceptionist) || !desk.HasRole(store.RoleTrainer) {
		t.Fatalf("multi roles = %v", desk.Roles)
	}

	r = e.do(http.MethodPost, "/api/auth/login", map[string]string{"email": desk.Email, "password": "password123"}, nil)
	e.want(r, http.StatusOK, "multi-role login")
	deskCookie = r.cookie

	r = e.do(http.MethodGet, "/api/payments", nil, deskCookie)
	e.want(r, http.StatusOK, "receptionist+trainer can list payments")

	r = e.do(http.MethodDelete, "/api/users/"+desk.ID.String(), nil, ownerCookie)
	e.want(r, http.StatusNoContent, "owner deletes receptionist")
}
