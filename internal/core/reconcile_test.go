package core

import (
	"strings"
	"testing"
	"time"
)

func timeAt(s string) time.Time { t, _ := time.Parse("2006-01-02", s); return t }
func base() Database {
	d := NewDatabase()
	d.Contracts = []Contract{{CustomerID: "c1", CustomerName: "Acme", Metric: "calls", Currency: "USD", UnitPriceCents: 10, IncludedUnits: 100, EffectiveFrom: "2026-01"}}
	d.Usage = []Usage{{ID: "u1", CustomerID: "c1", Period: "2026-08", Metric: "calls", Quantity: 300}}
	d.Billing = []Billing{{ID: "b1", InvoiceID: "in_1", CustomerID: "c1", Period: "2026-08", Metric: "calls", Quantity: 150, AmountCents: 1500, Currency: "USD", Status: "paid"}}
	return d
}
func TestScanDetectsAndPreservesStatus(t *testing.T) {
	d := base()
	s := Scan(&d, timeAt("2026-10-08"), 7)
	if s.Findings != 1 || len(d.Findings) != 1 {
		t.Fatalf("wrong findings: %+v", s)
	}
	f := d.Findings[0]
	if f.PotentialCents != 500 || f.Kind != "unbilled_usage" || f.ExpectedCents != 2000 {
		t.Fatalf("wrong finding: %+v", f)
	}
	d.Findings[0].Status = "reviewing"
	d.Findings[0].Note = "Contract verified"
	Scan(&d, timeAt("2026-10-09"), 7)
	if d.Findings[0].Status != "reviewing" || d.Findings[0].Note != "Contract verified" {
		t.Fatalf("lost status: %+v", d.Findings[0])
	}
}
func TestCreditsAndMinimumAndExcludedDraft(t *testing.T) {
	d := base()
	d.Contracts[0].MonthlyMinimumCents = 3000
	d.Billing[0].Status = "draft"
	d.Credits = []Credit{{ID: "cr1", CustomerID: "c1", Period: "2026-08", Metric: "calls", AmountCents: 750, Reason: "approved"}}
	Scan(&d, timeAt("2026-10-08"), 7)
	f := d.Findings[0]
	if f.Kind != "missing_invoice" || f.ExpectedCents != 2250 || f.PotentialCents != 2250 {
		t.Fatalf("wrong credit/minimum handling: %+v", f)
	}
}
func TestGraceAndCurrencyChecks(t *testing.T) {
	d := base()
	d.Usage[0].Period = "2026-09"
	d.Billing[0].Period = "2026-09"
	s := Scan(&d, timeAt("2026-10-06"), 7)
	if s.Findings != 0 || s.SkippedOpenPeriods != 1 {
		t.Fatalf("premature finding: %+v", s)
	}
	d.Billing[0].Currency = "EUR"
	s = Scan(&d, timeAt("2026-10-08"), 7)
	if s.Findings != 0 || s.SkippedCurrencies != 1 {
		t.Fatalf("currency mismatch: %+v", s)
	}
}
func TestMissingContract(t *testing.T) {
	d := base()
	d.Contracts = nil
	s := Scan(&d, timeAt("2026-10-08"), 7)
	if s.SkippedNoContract != 1 || s.Findings != 0 {
		t.Fatalf("unexpected scan: %+v", s)
	}
}
func TestNoDoubleCountOnReimport(t *testing.T) {
	d := NewDatabase()
	parsed, e := ReadCSV("usage", strings.NewReader("usage_id,customer_id,period,metric,quantity\na,c1,2026-08,calls,500\n"))
	if e != nil {
		t.Fatal(e)
	}
	a := Import(&d, "usage", parsed)
	b := Import(&d, "usage", parsed)
	if a.Added != 1 || b.Updated != 1 || len(d.Usage) != 1 {
		t.Fatalf("nonidempotent import: %v %v %+v", a, b, d.Usage)
	}
}
func TestRejectInvalidImport(t *testing.T) {
	tests := []struct{ kind, csv string }{
		{"usage", "usage_id,customer_id,period,metric,quantity\na,c1,2026-99,calls,10\n"},
		{"usage", "usage_id,customer_id,period,metric,quantity\na,c1,2026-08,calls,-1\n"},
		{"contracts", "customer_id,customer_name,metric,currency,unit_price_cents,included_units,monthly_minimum_cents,effective_from\nc1,A,calls,JPY,10,0,0,2026-01\n"},
		{"billing", "line_id,invoice_id,customer_id,period,metric,quantity,amount_cents,currency,status\nl1,i1,c1,2026-08,calls,20,300,USD,unpaid\n"},
	}
	for _, tt := range tests {
		if _, err := ReadCSV(tt.kind, strings.NewReader(tt.csv)); err == nil {
			t.Errorf("accepted invalid %s CSV: %s", tt.kind, tt.csv)
		}
	}
}
func TestStoreRollbackOnError(t *testing.T) {
	s, e := Open(t.TempDir() + "/store.json")
	if e != nil {
		t.Fatal(e)
	}
	_ = s.Update(func(d *Database) error { d.Contracts = append(d.Contracts, Contract{CustomerID: "keep"}); return nil })
	err := s.Update(func(d *Database) error { d.Contracts[0].CustomerID = "lost"; return errSentinel{} })
	if err == nil || s.Snapshot().Contracts[0].CustomerID != "keep" {
		t.Fatal("transaction was not rolled back")
	}
}

type errSentinel struct{}

func (errSentinel) Error() string { return "rejected" }
