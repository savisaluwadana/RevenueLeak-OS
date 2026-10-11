package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"revenueleak-os/internal/core"
)

func TestCoverageRoutePrivateAndSeparateFromRevenue(t *testing.T) {
	s := setup(t)
	p := time.Now().UTC().AddDate(0, -2, 0).Format("2006-01")
	if err := s.Store.Update(func(d *core.Database) error {
		d.Contracts = []core.Contract{{CustomerID: "c1", CustomerName: "Acme", Metric: "calls", Currency: "USD", MonthlyMinimumCents: 5000, EffectiveFrom: p, EffectiveTo: p}}
		core.RunScan(d, time.Now().UTC(), 7, "manual")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	h := s.Router()
	unauth := httptest.NewRecorder()
	h.ServeHTTP(unauth, httptest.NewRequest("GET", "/api/coverage", nil))
	if unauth.Code != 401 {
		t.Fatalf("coverage publicly accessible: %d", unauth.Code)
	}
	req := httptest.NewRequest("GET", "/api/coverage", nil)
	req.AddCookie(makeCookie(s))
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	if res.Code != 200 {
		t.Fatalf("coverage API failed: %d %s", res.Code, res.Body.String())
	}
	var body struct {
		Gaps []core.CoverageGap `json:"gaps"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Gaps) != 1 || body.Gaps[0].MonthlyMinimumCents != 5000 {
		t.Fatalf("wrong gaps: %+v", body)
	}
	snapshot := s.Store.Snapshot()
	if len(snapshot.Findings) != 0 || snapshot.LastScan.Findings != 0 || snapshot.LastScan.CoverageGaps != 1 {
		t.Fatalf("coverage inflated revenue: %+v", snapshot.LastScan)
	}
}
