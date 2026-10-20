package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CookieName is the session cookie.
const CookieName = "gym_session"

type ctxKey int

const sessionKey ctxKey = iota

// Session is the authenticated principal placed in the request context.
type Session struct {
	UserID    uuid.UUID
	TenantID  uuid.UUID
	Roles     []string
	TrainerID uuid.UUID // set when the login is linked to a trainer
}

// Has reports whether the session includes this role.
func (s Session) Has(role string) bool {
	for _, r := range s.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasAny is true when the session includes at least one of the given roles.
func (s Session) HasAny(roles ...string) bool {
	for _, role := range roles {
		if s.Has(role) {
			return true
		}
	}
	return false
}

// FromContext returns the session, if any.
func FromContext(ctx context.Context) (Session, bool) {
	s, ok := ctx.Value(sessionKey).(Session)
	return s, ok
}

// WithSession returns a context carrying the session (used by tests).
func WithSession(ctx context.Context, s Session) context.Context {
	return context.WithValue(ctx, sessionKey, s)
}

// Middleware rejects requests without a valid session token and otherwise
// injects the Session into the context. Accepts the cookie or a Bearer header.
func Middleware(tokens *Tokens) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := tokenFromRequest(r)
			if raw == "" {
				http.Error(w, `{"error":"unauthenticated"}`, http.StatusUnauthorized)
				return
			}
			claims, err := tokens.Verify(raw)
			if err != nil {
				http.Error(w, `{"error":"unauthenticated"}`, http.StatusUnauthorized)
				return
			}
			uid, err := claims.UserID()
			if err != nil {
				http.Error(w, `{"error":"unauthenticated"}`, http.StatusUnauthorized)
				return
			}
			s := Session{UserID: uid, TenantID: claims.TenantID, Roles: claims.RoleList(), TrainerID: claims.TrainerID}
			next.ServeHTTP(w, r.WithContext(WithSession(r.Context(), s)))
		})
	}
}

func tokenFromRequest(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	if c, err := r.Cookie(CookieName); err == nil {
		return c.Value
	}
	return ""
}

// SetSessionCookie writes the httpOnly session cookie.
func SetSessionCookie(w http.ResponseWriter, token string, ttl time.Duration, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
	})
}

// ClearSessionCookie expires the session cookie.
func ClearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
