package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"revenueleak-os/internal/core"
)

func recoveryRequest(t *testing.T, s *Server, method, path string, cookie *http.Cookie, csrf string, input any) *httptest.ResponseRecorder {
	t.Helper()
	var body []byte
	if input != nil {
		var err error
		body, err = json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	res := httptest.NewRecorder()
	s.Router().ServeHTTP(res, req)
	return res
}

func TestRecoveryRoutesRequireSessionCSRFAndEvidence(t *testing.T) {
	s := setup(t)
	if err := s.Store.Update(func(d *core.Database) error {
		*d = core.NewDatabase()
		d.Contracts = []core.Contract{{CustomerID: "customer1", CustomerName: "Acme", Metric: "tokens", Currency: "USD", UnitPriceCents: 10, EffectiveFrom: "2026-01"}}
		d.Usage = []core.Usage{{ID: "use1", CustomerID: "customer1", Period: "2026-08", Metric: "tokens", Quantity: 100}}
		core.RunScan(d, time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC), 7, "test")
		if len(d.Findings) == 0 {
			t.Fatal("fixture has no finding")
		}
		d.Findings[0].Status = "approved"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	f := snapshot(t, s.Store).Findings[0]
	payload := core.RecoveryInput{IdempotencyKey: "recovery-case-0001", EvidenceDigest: f.EvidenceDigest, Action: "confirmed", AmountCents: 250, Note: "Reviewed signed rate card and invoice"}
	url := "/api/findings/" + f.ID + "/recovery"
	if res := recoveryRequest(t, s, "GET", "/api/recoveries", nil, "", nil); res.Code != 401 {
		t.Fatalf("unauthorized recovery disclosure: %d", res.Code)
	}
	if res := recoveryRequest(t, s, "POST", url, nil, "", payload); res.Code != 401 {
		t.Fatalf("unauthorized recovery write: %d", res.Code)
	}
	cookie := makeCookie(s)
	if res := recoveryRequest(t, s, "POST", url, cookie, "", payload); res.Code != 403 {
		t.Fatalf("CSRF bypassed: %d", res.Code)
	}
	csrf := s.sign("csrf:" + cookie.Value)
	res := recoveryRequest(t, s, "POST", url, cookie, csrf, payload)
	if res.Code != 200 {
		t.Fatalf("valid recovery record: %d %s", res.Code, res.Body.String())
	}
	res = recoveryRequest(t, s, "POST", url, cookie, csrf, payload)
	if res.Code != 200 || len(snapshot(t, s.Store).Recoveries) != 1 {
		t.Fatalf("idempotent retry failed: %d %s", res.Code, res.Body.String())
	}
	res = recoveryRequest(t, s, "GET", "/api/recoveries", cookie, "", nil)
	if res.Code != 200 {
		t.Fatalf("recovery audit read failed: %d %s", res.Code, res.Body.String())
	}
	var result struct {
		Events []core.RecoveryEntry                `json:"events"`
		Totals map[string]core.RecoveryCurrencyTotals `json:"totals_by_currency"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Events) != 1 || result.Totals["USD"].ConfirmedCurrentCents != 250 {
		t.Fatalf("incorrect recovery audit: %+v", result)
	}
	payload.Action = "collected"
	payload.AmountCents = 300
	payload.IdempotencyKey = "recovery-case-0002"
	if res := recoveryRequest(t, s, "POST", url, cookie, csrf, payload); res.Code != 400 {
		t.Fatalf("unconfirmed collection accepted: %d %s", res.Code, res.Body.String())
	}
	payload.EvidenceDigest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if res := recoveryRequest(t, s, "POST", url, cookie, csrf, payload); res.Code != 409 {
		t.Fatalf("stale financial evidence accepted: %d %s", res.Code, res.Body.String())
	}
}

func TestRecoveryReadFailsClosedWithStorageOutage(t *testing.T) {
	s := New(failingStorage{}, "secret", false, "../../web")
	res := recoveryRequest(t, s, "GET", "/api/recoveries", makeCookie(s), "", nil)
	if res.Code != 500 {
		t.Fatalf("recovery read suppressed database failure: %d %s", res.Code, res.Body.String())
	}
}
