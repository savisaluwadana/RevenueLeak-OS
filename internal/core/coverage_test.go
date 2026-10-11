package core

import "testing"

func contractWithMinimum(start string) Contract {
	return Contract{CustomerID: "c1", CustomerName: "Acme", Metric: "calls", Currency: "USD", UnitPriceCents: 10, MonthlyMinimumCents: 3000, EffectiveFrom: start}
}

func TestCoverageGraceAndNoInventedRevenue(t *testing.T) {
	d := NewDatabase()
	d.Contracts = []Contract{contractWithMinimum("2026-09")}
	if s := Scan(&d, timeAt("2026-10-07"), 7); s.CoverageGaps != 0 {
		t.Fatalf("premature coverage: %+v", s)
	}
	s := Scan(&d, timeAt("2026-10-08"), 7)
	if s.Findings != 0 || s.CoverageGaps != 1 || len(d.Coverage) != 1 {
		t.Fatalf("coverage conflated with a revenue claim: %+v", s)
	}
	g := d.Coverage[0]
	if g.ID == "" || g.Period != "2026-09" || g.MonthlyMinimumCents != 3000 || g.Explanation == "" {
		t.Fatalf("incomplete coverage record: %+v", g)
	}
}

func TestCoverageRespectsFinalizedBillingAndUsage(t *testing.T) {
	d := NewDatabase()
	d.Contracts = []Contract{contractWithMinimum("2026-09")}
	d.Billing = []Billing{{ID: "b", InvoiceID: "i", CustomerID: "c1", Period: "2026-09", Metric: "calls", Currency: "USD", Status: "draft"}}
	Scan(&d, timeAt("2026-10-08"), 7)
	if len(d.Coverage) != 1 {
		t.Fatal("draft invoice does not establish final billing")
	}
	d.Billing[0].Status = "paid"
	Scan(&d, timeAt("2026-10-08"), 7)
	if len(d.Coverage) != 0 {
		t.Fatal("paid invoice should close both-sources-missing signal")
	}
	d.Billing = nil
	d.Usage = []Usage{{ID: "u", CustomerID: "c1", Period: "2026-09", Metric: "calls", Quantity: 8}}
	Scan(&d, timeAt("2026-10-08"), 7)
	if len(d.Coverage) != 0 {
		t.Fatal("usage should close both-sources-missing signal")
	}
}

func TestCoverageCreditAndContractVersion(t *testing.T) {
	d := NewDatabase()
	older := contractWithMinimum("2026-08")
	older.EffectiveTo = "2026-08"
	newer := contractWithMinimum("2026-09")
	newer.MonthlyMinimumCents = 0
	d.Contracts = []Contract{older, newer}
	d.Credits = []Credit{{ID: "cr", CustomerID: "c1", Period: "2026-08", Metric: "calls", AmountCents: 3000, Reason: "waiver"}}
	s := Scan(&d, timeAt("2026-10-08"), 7)
	if s.CoverageGaps != 1 || s.Findings != 0 || d.Coverage[0].Period != "2026-08" {
		t.Fatalf("credits, newer terms or missing inputs distorted coverage: %+v %+v", s, d.Coverage)
	}
}

func TestCoverageBoundedAndSavedRunsImmutable(t *testing.T) {
	d := NewDatabase()
	d.Contracts = []Contract{contractWithMinimum("2020-01")}
	RunScan(&d, timeAt("2026-10-08"), 7, "manual")
	if len(d.Coverage) != CoverageLookbackMonths || d.Coverage[0].Period != "2026-09" || d.Coverage[len(d.Coverage)-1].Period != "2025-10" {
		t.Fatalf("incorrect lookback: %+v", d.Coverage)
	}
	saved := d.ScanRuns[0].Coverage[0].ID
	d.Contracts = nil
	RunScan(&d, timeAt("2026-10-09"), 7, "import.contracts")
	if len(d.Coverage) != 0 || len(d.ScanRuns[1].Coverage) != CoverageLookbackMonths || d.ScanRuns[1].Coverage[0].ID != saved {
		t.Fatal("saved coverage changed after new import")
	}
}
