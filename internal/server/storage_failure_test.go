package server

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"revenueleak-os/internal/core"
)

// failingStorage simulates a database outage; no user-facing route should
// silently render a zero-dollar portfolio or a missing-run 404.
type failingStorage struct{}

func (failingStorage) Read() (core.Database, error) {
	return core.Database{}, errors.New("PRIVATE DATABASE CREDENTIALS: do not return")
}
func (failingStorage) Update(fn func(*core.Database) error) error {
	return errors.New("backend unavailable")
}

func TestPrivateReadAndExportEndpointsFailClosed(t *testing.T) {
	s := New(failingStorage{}, "secret", false, "../../web")
	cookie := makeCookie(s)
	h := s.Router()
	paths := []string{
		"/api/overview",
		"/api/findings",
		"/api/coverage",
		"/api/datasets",
		"/api/audit",
		"/api/findings/export",
		"/api/scans",
		"/api/scans/test",
		"/api/scans/test/export",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			req.AddCookie(cookie)
			out := httptest.NewRecorder()
			h.ServeHTTP(out, req)
			if out.Code != 500 {
				t.Fatalf("read error should be 500, got %d: %s", out.Code, out.Body.String())
			}
			if !strings.Contains(out.Body.String(), "Unable to load workspace data") {
				t.Fatalf("read error message is misleading: %s", out.Body.String())
			}
			if strings.Contains(out.Body.String(), "PRIVATE DATABASE CREDENTIALS") {
				t.Fatal("internal database error leaked to browser")
			}
			if out.Header().Get("Content-Disposition") != "" {
				t.Fatal("failed financial export must not advertise a file")
			}
		})
	}
}

func TestUnauthenticatedReadStillDeniedBeforeDatabaseAccess(t *testing.T) {
	s := New(failingStorage{}, "secret", false, "../../web")
	out := httptest.NewRecorder()
	s.Router().ServeHTTP(out, httptest.NewRequest("GET", "/api/overview", nil))
	if out.Code != 401 {
		t.Fatalf("storage errors must not bypass authentication, got %d", out.Code)
	}
}
