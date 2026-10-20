// Package auth implements password hashing, session tokens and the HTTP
// middleware that resolves the current tenant from the session.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims is the JWT payload. The tenant id lives in the signed token, so a
// client can never choose which tenant it operates on.
type Claims struct {
	TenantID  uuid.UUID `json:"tid"`
	Roles     []string  `json:"roles"`
	Role      string    `json:"role,omitempty"` // legacy single-role tokens
	TrainerID uuid.UUID `json:"trid,omitempty"`
	jwt.RegisteredClaims
}

// Tokens signs and verifies session JWTs.
type Tokens struct {
	secret []byte
	ttl    time.Duration
}

// NewTokens builds a token signer with the given HMAC secret and lifetime.
func NewTokens(secret string, ttl time.Duration) *Tokens {
	return &Tokens{secret: []byte(secret), ttl: ttl}
}

// TTL returns the configured token lifetime.
func (t *Tokens) TTL() time.Duration { return t.ttl }

// Issue creates a signed token for the user/tenant pair. trainerID is uuid.Nil
// except when the login is linked to a trainers row.
func (t *Tokens) Issue(userID, tenantID uuid.UUID, roles []string, trainerID uuid.UUID) (string, error) {
	now := time.Now()
	legacy := ""
	if len(roles) > 0 {
		legacy = roles[0]
	}
	claims := Claims{
		TenantID:  tenantID,
		Roles:     roles,
		Role:      legacy,
		TrainerID: trainerID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(t.ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
}

// RoleList returns the roles on the token, mapping a legacy cashier claim.
func (c *Claims) RoleList() []string {
	src := c.Roles
	if len(src) == 0 && c.Role != "" {
		src = []string{c.Role}
	}
	out := make([]string, 0, len(src))
	seen := map[string]struct{}{}
	for _, r := range src {
		if r == "cashier" {
			r = "receptionist"
		}
		if r == "" {
			continue
		}
		if _, ok := seen[r]; ok {
			continue
		}
		seen[r] = struct{}{}
		out = append(out, r)
	}
	return out
}

// ErrInvalidToken is returned for any token that fails verification.
var ErrInvalidToken = errors.New("invalid session token")

// Verify parses and validates a token, returning its claims.
func (t *Tokens) Verify(raw string) (*Claims, error) {
	claims := &Claims{}
	tok, err := jwt.ParseWithClaims(raw, claims, func(tok *jwt.Token) (any, error) {
		if _, ok := tok.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", tok.Header["alg"])
		}
		return t.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !tok.Valid {
		return nil, ErrInvalidToken
	}
	if claims.TenantID == uuid.Nil || claims.Subject == "" {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// UserID parses the subject claim.
func (c *Claims) UserID() (uuid.UUID, error) {
	return uuid.Parse(c.Subject)
}
