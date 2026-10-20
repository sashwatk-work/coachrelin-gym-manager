package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "correct horse battery" {
		t.Fatal("hash must not equal the plaintext")
	}
	if !CheckPassword(hash, "correct horse battery") {
		t.Error("expected matching password to verify")
	}
	if CheckPassword(hash, "wrong password") {
		t.Error("expected wrong password to fail")
	}
	if CheckPassword("not-a-bcrypt-hash", "anything") {
		t.Error("expected malformed hash to fail")
	}
}

func TestHashPasswordRejectsShort(t *testing.T) {
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("expected error for password shorter than 8 characters")
	}
}

func TestTokensRoundTrip(t *testing.T) {
	tokens := NewTokens("unit-test-secret", time.Hour)
	userID, tenantID := uuid.New(), uuid.New()

	raw, err := tokens.Issue(userID, tenantID, []string{"owner"}, uuid.Nil)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	claims, err := tokens.Verify(raw)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	gotUser, err := claims.UserID()
	if err != nil {
		t.Fatalf("UserID: %v", err)
	}
	if gotUser != userID {
		t.Errorf("user id = %s, want %s", gotUser, userID)
	}
	if claims.TenantID != tenantID {
		t.Errorf("tenant id = %s, want %s", claims.TenantID, tenantID)
	}
	if got := strings.Join(claims.RoleList(), ","); got != "owner" {
		t.Errorf("roles = %q, want owner", got)
	}
	if tokens.TTL() != time.Hour {
		t.Errorf("TTL = %s, want 1h", tokens.TTL())
	}
}

func TestRoleListMapsLegacyCashier(t *testing.T) {
	c := &Claims{Role: "cashier"}
	if got := strings.Join(c.RoleList(), ","); got != "receptionist" {
		t.Errorf("RoleList = %q, want receptionist", got)
	}
	c = &Claims{Roles: []string{"owner", "cashier", "trainer"}}
	if got := strings.Join(c.RoleList(), ","); got != "owner,receptionist,trainer" {
		t.Errorf("RoleList = %q, want owner,receptionist,trainer", got)
	}
}

func TestTokensRejectBadTokens(t *testing.T) {
	tokens := NewTokens("unit-test-secret", time.Hour)
	raw, err := tokens.Issue(uuid.New(), uuid.New(), []string{"owner"}, uuid.Nil)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	t.Run("wrong secret", func(t *testing.T) {
		other := NewTokens("a-different-secret", time.Hour)
		if _, err := other.Verify(raw); err == nil {
			t.Fatal("expected token signed with another secret to be rejected")
		}
	})

	t.Run("tampered payload", func(t *testing.T) {
		parts := strings.Split(raw, ".")
		if len(parts) != 3 {
			t.Fatalf("unexpected token shape: %d parts", len(parts))
		}
		// Flip a character in the payload; the signature no longer matches.
		payload := []byte(parts[1])
		if payload[0] == 'A' {
			payload[0] = 'B'
		} else {
			payload[0] = 'A'
		}
		tampered := parts[0] + "." + string(payload) + "." + parts[2]
		if _, err := tokens.Verify(tampered); err == nil {
			t.Fatal("expected tampered token to be rejected")
		}
	})

	t.Run("expired", func(t *testing.T) {
		expired := NewTokens("unit-test-secret", -time.Minute)
		raw, err := expired.Issue(uuid.New(), uuid.New(), []string{"owner"}, uuid.Nil)
		if err != nil {
			t.Fatalf("Issue: %v", err)
		}
		if _, err := tokens.Verify(raw); err == nil {
			t.Fatal("expected expired token to be rejected")
		}
	})

	t.Run("garbage", func(t *testing.T) {
		if _, err := tokens.Verify("not.a.jwt"); err == nil {
			t.Fatal("expected garbage to be rejected")
		}
		if _, err := tokens.Verify(""); err == nil {
			t.Fatal("expected empty string to be rejected")
		}
	})

	t.Run("nil tenant", func(t *testing.T) {
		raw, err := tokens.Issue(uuid.New(), uuid.Nil, []string{"owner"}, uuid.Nil)
		if err != nil {
			t.Fatalf("Issue: %v", err)
		}
		if _, err := tokens.Verify(raw); err == nil {
			t.Fatal("expected token without tenant to be rejected")
		}
	})
}

func TestMiddleware(t *testing.T) {
	tokens := NewTokens("unit-test-secret", time.Hour)
	userID, tenantID := uuid.New(), uuid.New()
	raw, err := tokens.Issue(userID, tenantID, []string{"receptionist"}, uuid.Nil)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	var got Session
	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		got, _ = FromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})
	h := Middleware(tokens)(next)

	run := func(mod func(*http.Request)) *httptest.ResponseRecorder {
		called = false
		got = Session{}
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		mod(req)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	t.Run("no token", func(t *testing.T) {
		rec := run(func(*http.Request) {})
		if rec.Code != http.StatusUnauthorized || called {
			t.Fatalf("status = %d, called = %v; want 401 and not called", rec.Code, called)
		}
	})

	t.Run("bearer header", func(t *testing.T) {
		rec := run(func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+raw) })
		if rec.Code != http.StatusNoContent || !called {
			t.Fatalf("status = %d, called = %v; want 204 and called", rec.Code, called)
		}
		if got.UserID != userID || got.TenantID != tenantID || !got.Has("receptionist") {
			t.Errorf("session = %+v", got)
		}
	})

	t.Run("cookie", func(t *testing.T) {
		rec := run(func(r *http.Request) { r.AddCookie(&http.Cookie{Name: CookieName, Value: raw}) })
		if rec.Code != http.StatusNoContent || !called {
			t.Fatalf("status = %d, called = %v; want 204 and called", rec.Code, called)
		}
		if got.TenantID != tenantID {
			t.Errorf("tenant = %s, want %s", got.TenantID, tenantID)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		rec := run(func(r *http.Request) { r.Header.Set("Authorization", "Bearer nope") })
		if rec.Code != http.StatusUnauthorized || called {
			t.Fatalf("status = %d, called = %v; want 401 and not called", rec.Code, called)
		}
	})

	t.Run("header wins over cookie", func(t *testing.T) {
		rec := run(func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer nope")
			r.AddCookie(&http.Cookie{Name: CookieName, Value: raw})
		})
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d; want 401 because the explicit header is invalid", rec.Code)
		}
	})
}

func TestSessionCookies(t *testing.T) {
	rec := httptest.NewRecorder()
	SetSessionCookie(rec, "tok", 2*time.Hour, true)
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies, want 1", len(cookies))
	}
	c := cookies[0]
	if c.Name != CookieName || c.Value != "tok" {
		t.Errorf("cookie = %s=%s", c.Name, c.Value)
	}
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
		t.Errorf("cookie attributes: httponly=%v secure=%v samesite=%v path=%q", c.HttpOnly, c.Secure, c.SameSite, c.Path)
	}
	if c.MaxAge != 7200 {
		t.Errorf("MaxAge = %d, want 7200", c.MaxAge)
	}

	rec = httptest.NewRecorder()
	ClearSessionCookie(rec, false)
	cookies = rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge >= 0 || cookies[0].Value != "" {
		t.Fatalf("clear cookie = %+v; want expired empty cookie", cookies)
	}
}
