package server

import (
	"encoding/json"
	"net/http/httptest"
	"revenueleak-os/internal/core"
	"strings"
	"testing"
	"time"
)

func TestSavedRunRoutesRequireSessionAndExportStableSnapshot(t *testing.T) {
	s := setup(t)
	if err := s.Store.Update(func(d *core.Database) error { core.SeedDemo(d, time.Now()); return nil }); err != nil {
		t.Fatal(err)
	}
	id := s.Store.Snapshot().ScanRuns[0].ID
	h := s.Router()
	for _, path := range []string{"/api/scans", "/api/scans/" + id, "/api/scans/" + id + "/export"} {
		res := httptest.NewRecorder()
		h.ServeHTTP(res, httptest.NewRequest("GET", path, nil))
		if res.Code != 401 {
			t.Fatalf("unauthorized read %s: %d", path, res.Code)
		}
	}
	req := httptest.NewRequest("POST", "/api/login", strings.NewReader(`{"password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	if res.Code != 200 {
		t.Fatal("login failed")
	}
	cookie := res.Result().Cookies()[0]
	get := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", path, nil)
		req.AddCookie(cookie)
		out := httptest.NewRecorder()
		h.ServeHTTP(out, req)
		return out
	}
	index := get("/api/scans")
	if index.Code != 200 || strings.Contains(index.Body.String(), `"sources"`) {
		t.Fatal("index should contain summaries only")
	}
	first := get("/api/scans/" + id + "/export")
	if first.Code != 200 || !strings.Contains(first.Header().Get("Content-Disposition"), "attachment;") || first.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("incorrect export headers")
	}
	var result struct {
		SchemaVersion string       `json:"schema_version"`
		Run           core.ScanRun `json:"run"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.SchemaVersion != "1" || result.Run.ID != id || len(result.Run.Sources.Contracts) == 0 {
		t.Fatal("incomplete evidence export")
	}
	if err := s.Store.Update(func(d *core.Database) error { d.Contracts[0].CustomerName = "Changed"; return nil }); err != nil {
		t.Fatal(err)
	}
	if get("/api/scans/"+id+"/export").Body.String() != first.Body.String() {
		t.Fatal("export changed after source correction")
	}
	if get("/api/scans/missing").Code != 404 {
		t.Fatal("missing run should be 404")
	}
}
