package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/db"
	"github.com/sashwatk-work/coachrelin-gym-manager/backend/internal/store"
)

const maxBodyBytes = 1 << 20 // 1 MiB

// ValidationError carries per-field messages and renders as HTTP 400.
type ValidationError struct {
	Fields map[string]string
}

func (v *ValidationError) Error() string {
	parts := make([]string, 0, len(v.Fields))
	for k, m := range v.Fields {
		parts = append(parts, k+": "+m)
	}
	return "validation failed: " + strings.Join(parts, "; ")
}

// validator accumulates field errors.
type validator struct{ fields map[string]string }

func newValidator() *validator { return &validator{fields: map[string]string{}} }

func (v *validator) require(field, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		v.fields[field] = "is required"
	}
	return value
}

func (v *validator) add(field, msg string) { v.fields[field] = msg }

func (v *validator) oneOf(field, value string, allowed ...string) {
	for _, a := range allowed {
		if a == value {
			return
		}
	}
	v.fields[field] = "must be one of: " + strings.Join(allowed, ", ")
}

func (v *validator) err() error {
	if len(v.fields) == 0 {
		return nil
	}
	return &ValidationError{Fields: v.fields}
}

// httpError is an error with an explicit status.
type httpError struct {
	status int
	msg    string
}

func (e *httpError) Error() string { return e.msg }

func errStatus(status int, msg string) error { return &httpError{status: status, msg: msg} }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string, fields map[string]string) {
	body := map[string]any{"error": msg}
	if len(fields) > 0 {
		body["fields"] = fields
	}
	writeJSON(w, status, body)
}

// decode reads a JSON body into v.
func decode(r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return errStatus(http.StatusBadRequest, "request body is required")
		}
		return errStatus(http.StatusBadRequest, "invalid JSON: "+err.Error())
	}
	return nil
}

// respondErr maps errors to HTTP responses.
func (s *Server) respondErr(w http.ResponseWriter, r *http.Request, err error) {
	var ve *ValidationError
	var he *httpError
	switch {
	case errors.As(err, &ve):
		writeError(w, http.StatusBadRequest, "validation failed", ve.Fields)
	case errors.As(err, &he):
		writeError(w, he.status, he.msg, nil)
	case errors.Is(err, db.ErrTenantNotFound):
		writeError(w, http.StatusUnauthorized, "unauthenticated", nil)
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found", nil)
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", nil)
	default:
		s.log.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
		writeError(w, http.StatusInternalServerError, "internal error", nil)
	}
}

// urlUUID parses a {param} path segment as a UUID.
func urlUUID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		return uuid.Nil, errStatus(http.StatusBadRequest, fmt.Sprintf("invalid %s", name))
	}
	return id, nil
}

// listParams extracts q/status/page/page_size.
func listParams(r *http.Request) store.ListParams {
	qs := r.URL.Query()
	p := store.ListParams{
		Q:      qs.Get("q"),
		Status: qs.Get("status"),
	}
	p.Page, _ = strconv.Atoi(qs.Get("page"))
	p.PageSize, _ = strconv.Atoi(qs.Get("page_size"))
	p.Normalize()
	return p
}

// optUUID parses an optional query param as UUID.
func optUUID(r *http.Request, name string) (*uuid.UUID, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, errStatus(http.StatusBadRequest, "invalid "+name)
	}
	return &id, nil
}

// optDate parses an optional query param as YYYY-MM-DD.
func optDate(r *http.Request, name string) (*store.Date, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return nil, nil
	}
	d, err := store.ParseDate(raw)
	if err != nil {
		return nil, errStatus(http.StatusBadRequest, "invalid "+name+": expected YYYY-MM-DD")
	}
	return &d, nil
}

// strPtr returns nil for blank strings, else a pointer to the trimmed value.
func strPtr(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}
