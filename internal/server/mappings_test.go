package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"revenueleak-os/internal/core"
)

func TestMappingImportRequiresLoginAndReconcilesProviderIDs(t *testing.T) {
	s := setup(t)
	if err := s.Store.Update(func(d *core.Database) error {
		d.Contracts = []core.Contract{{CustomerID: "customer-contract", CustomerName: "Acme", Metric: "calls", Currency: "USD", UnitPriceCents: 10, EffectiveFrom: "2026-01"}}
		d.Billing = []core.Billing{{ID: "l1", InvoiceID: "inv1", CustomerID: "cus_external", Period: "2026-08", Metric: "calls", Quantity: 1, AmountCents: 5, Currency: "USD", Status: "paid", Source: "stripe"}}
		d.Usage = []core.Usage{{ID: "u1", CustomerID: "customer-contract", Period: "2026-08", Metric: "calls", Quantity: 100}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	file, err := writer.CreateFormFile("file", "mappings.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write([]byte("source,external_customer_id,customer_id\nstripe,cus_external,customer-contract\n")); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	h := s.Router()
	req := httptest.NewRequest("POST", "/api/import/mappings", bytes.NewReader(body.Bytes()))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("mapping import was public: %d", rec.Code)
	}
	cookie := makeCookie(s)
	req = httptest.NewRequest("POST", "/api/import/mappings", bytes.NewReader(body.Bytes()))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.AddCookie(cookie)
	req.Header.Set("X-CSRF-Token", s.sign("csrf:"+cookie.Value))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("authorized mapping import failed: %d %s", rec.Code, rec.Body.String())
	}
	snapshot := snapshot(t, s.Store)
	if len(snapshot.Mappings) != 1 || snapshot.LastScan.Findings != 1 || snapshot.Findings[0].CustomerID != "customer-contract" || snapshot.Findings[0].BilledCents != 5 {
		t.Fatalf("mapping was not used for reconciliation: %+v", snapshot)
	}
	req = httptest.NewRequest("GET", "/api/datasets", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatal("authenticated datasets unavailable")
	}
	var dataset struct {
		Mappings []core.CustomerMapping `json:"mappings"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &dataset); err != nil {
		t.Fatal(err)
	}
	if len(dataset.Mappings) != 1 || dataset.Mappings[0].ExternalCustomerID != "cus_external" {
		t.Fatalf("mapping source evidence missing: %+v", dataset)
	}
}
