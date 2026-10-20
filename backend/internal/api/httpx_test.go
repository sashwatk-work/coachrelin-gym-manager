package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/db"
	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/store"
)

func quietServer() *Server {
	return &Server{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func TestValidator(t *testing.T) {
	v := newValidator()
	if got := v.require("name", "  Relin  "); got != "Relin" {
		t.Errorf("require trimmed = %q, want Relin", got)
	}
	v.require("empty", "   ")
	v.oneOf("status", "frozen", "active", "frozen", "inactive")
	v.oneOf("method", "bitcoin", "cash", "card")
	err := v.err()
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want *ValidationError", err)
	}
	if ve.Fields["empty"] != "is required" {
		t.Errorf("empty field message = %q", ve.Fields["empty"])
	}
	if _, ok := ve.Fields["status"]; ok {
		t.Error("valid oneOf value must not produce an error")
	}
	if !strings.HasPrefix(ve.Fields["method"], "must be one of") {
		t.Errorf("method message = %q", ve.Fields["method"])
	}
	if newValidator().err() != nil {
		t.Error("empty validator must return nil error")
	}
}

func TestRespondErrMapping(t *testing.T) {
	s := quietServer()
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantMsg    string
	}{
		{"validation", &ValidationError{Fields: map[string]string{"email": "is required"}}, http.StatusBadRequest, "validation failed"},
		{"http error", errStatus(http.StatusTeapot, "short and stout"), http.StatusTeapot, "short and stout"},
		{"tenant gone", db.ErrTenantNotFound, http.StatusUnauthorized, "unauthenticated"},
		{"not found", store.ErrNotFound, http.StatusNotFound, "not found"},
		{"conflict", store.ErrConflict, http.StatusConflict, "conflict"},
		{"wrapped not found", errors.Join(errors.New("ctx"), store.ErrNotFound), http.StatusNotFound, "not found"},
		{"unknown", errors.New("boom"), http.StatusInternalServerError, "internal error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			s.respondErr(rec, req, tc.err)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			var body map[string]any
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["error"] != tc.wantMsg {
				t.Errorf("error = %v, want %q", body["error"], tc.wantMsg)
			}
			if tc.name == "validation" {
				fields, _ := body["fields"].(map[string]any)
				if fields["email"] != "is required" {
					t.Errorf("fields = %v", body["fields"])
				}
			}
		})
	}
}

func TestDecode(t *testing.T) {
	var v struct {
		Name string `json:"name"`
	}

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"ok"}`))
	if err := decode(req, &v); err != nil || v.Name != "ok" {
		t.Fatalf("decode valid: err=%v name=%q", err, v.Name)
	}

	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	err := decode(req, &v)
	var he *httpError
	if !errors.As(err, &he) || he.status != http.StatusBadRequest || he.msg != "request body is required" {
		t.Fatalf("decode empty: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":`))
	err = decode(req, &v)
	if !errors.As(err, &he) || he.status != http.StatusBadRequest {
		t.Fatalf("decode malformed: %v", err)
	}
}

func TestListParams(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?q=ra&status=active&page=0&page_size=100000", nil)
	p := listParams(req)
	if p.Q != "ra" || p.Status != "active" {
		t.Errorf("q/status = %q/%q", p.Q, p.Status)
	}
	if p.Page != 1 {
		t.Errorf("page = %d, want 1 (clamped)", p.Page)
	}
	if p.PageSize <= 0 || p.PageSize >= 100000 {
		t.Errorf("page_size = %d, want clamped to a sane maximum", p.PageSize)
	}
}

func TestStrPtr(t *testing.T) {
	if strPtr(nil) != nil {
		t.Error("nil in, nil out")
	}
	blank := "   "
	if strPtr(&blank) != nil {
		t.Error("blank string must become nil")
	}
	val := "  x "
	if got := strPtr(&val); got == nil || *got != "x" {
		t.Errorf("got %v, want x", got)
	}
}

func TestUnauthenticatedRoutesWithoutDB(t *testing.T) {
	// Protected routes reject before touching the database, so a Server with
	// no DB is enough to verify the middleware wiring on every group.
	s := quietServer()
	s.tokens = newTestTokens()
	h := s.Router()

	paths := []string{
		"/api/auth/me", "/api/dashboard", "/api/settings/tenant", "/api/users", "/api/members", "/api/trainers",
		"/api/plans", "/api/payments", "/api/pt/plans", "/api/pt/packages", "/api/pt/sessions",
	}
	for _, p := range paths {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("GET %s without session = %d, want 401", p, rec.Code)
		}
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("healthz = %d, want 200", rec.Code)
	}
}
