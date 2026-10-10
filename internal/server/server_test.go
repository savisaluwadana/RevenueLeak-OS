package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"revenueleak-os/internal/core"
)

func setup(t *testing.T) *Server {
	t.Helper()
	store, err := core.Open(t.TempDir() + "/db.json")
	if err != nil {
		t.Fatal(err)
	}
	return New(store, "secret", false, "../../web")
}
func TestAuthAndCSRF(t *testing.T) {
	s := setup(t)
	h := s.Router()
	req := httptest.NewRequest("GET", "/api/findings", nil)
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	if res.Code != 401 {
		t.Fatalf("unauthenticated GET: %d", res.Code)
	}
	req = httptest.NewRequest("POST", "/api/login", strings.NewReader(`{"password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	res = httptest.NewRecorder()
	h.ServeHTTP(res, req)
	if res.Code != 200 {
		t.Fatalf("login: %d %s", res.Code, res.Body.String())
	}
	cookies := res.Result().Cookies()
	if len(cookies) == 0 || !cookies[0].HttpOnly {
		t.Fatal("missing secure session attributes")
	}
	var result map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest("POST", "/api/scan", nil)
	req.AddCookie(cookies[0])
	res = httptest.NewRecorder()
	h.ServeHTTP(res, req)
	if res.Code != 403 {
		t.Fatalf("csrf not enforced: %d", res.Code)
	}
	req = httptest.NewRequest("POST", "/api/scan", nil)
	req.AddCookie(cookies[0])
	req.Header.Set("X-CSRF-Token", result["csrf"].(string))
	res = httptest.NewRecorder()
	h.ServeHTTP(res, req)
	if res.Code != 200 {
		t.Fatalf("scan failed: %d %s", res.Code, res.Body.String())
	}
}
func TestDemoCannotResetInProduction(t *testing.T) {
	s := setup(t)
	req := httptest.NewRequest("POST", "/api/demo/reset", nil)
	req.AddCookie(makeCookie(s))
	req.Header.Set("X-CSRF-Token", s.sign("csrf:"+makeCookie(s).Value))
	res := httptest.NewRecorder()
	s.Router().ServeHTTP(res, req)
	if res.Code != 403 {
		t.Fatalf("reset enabled: %d", res.Code)
	}
}
func makeCookie(s *Server) *http.Cookie {
	login := httptest.NewRequest("POST", "/api/login", strings.NewReader(`{"password":"secret"}`))
	login.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	s.Router().ServeHTTP(res, login)
	return res.Result().Cookies()[0]
}
func TestUploadAndFindings(t *testing.T) {
	s := setup(t)
	_ = s.Store.Update(func(d *core.Database) error {
		d.Contracts = append(d.Contracts, core.Contract{CustomerID: "c1", CustomerName: "Acme", Metric: "calls", Currency: "USD", UnitPriceCents: 10, EffectiveFrom: "2026-01"})
		return nil
	})
	c := makeCookie(s)
	buf := &bytes.Buffer{}
	writer := multipart.NewWriter(buf)
	file, _ := writer.CreateFormFile("file", "usage.csv")
	file.Write([]byte("usage_id,customer_id,period,metric,quantity\nu1,c1,2026-08,calls,500\n"))
	writer.Close()
	req := httptest.NewRequest("POST", "/api/import/usage", buf)
	req.AddCookie(c)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-CSRF-Token", s.sign("csrf:"+c.Value))
	res := httptest.NewRecorder()
	s.Router().ServeHTTP(res, req)
	if res.Code != 200 {
		t.Fatalf("upload: %d %s", res.Code, res.Body.String())
	}
	d := s.Store.Snapshot()
	if len(d.Usage) != 1 || len(d.Findings) != 1 || d.Findings[0].Kind != "missing_invoice" {
		t.Fatalf("unexpected data: %+v", d)
	}
}
