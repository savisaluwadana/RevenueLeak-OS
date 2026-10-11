package core

import (
	"strings"
	"testing"
)

func TestStripeAliasMatchesCanonicalContractWithoutChangingRawSource(t *testing.T) {
	d := base()
	d.Billing[0].Source = "stripe"
	d.Billing[0].CustomerID = "cus_external"
	Scan(&d, timeAt("2026-10-08"), 7)
	if len(d.Findings) != 1 || d.Findings[0].PotentialCents != 2000 {
		t.Fatalf("unmapped Stripe invoice should not silently match a contract: %+v", d.Findings)
	}
	d.Mappings = []CustomerMapping{{Source: "billing_csv", ExternalCustomerID: "cus_external", CustomerID: "c1"}}
	Scan(&d, timeAt("2026-10-08"), 7)
	if d.Findings[0].PotentialCents != 2000 {
		t.Fatal("mapping for billing_csv must never apply to Stripe")
	}
	d.Mappings = []CustomerMapping{{Source: "stripe", ExternalCustomerID: "cus_external", CustomerID: "c1"}}
	Scan(&d, timeAt("2026-10-08"), 7)
	if len(d.Findings) != 1 || d.Findings[0].PotentialCents != 500 || d.Findings[0].BilledCents != 1500 || d.Findings[0].CustomerID != "c1" {
		t.Fatalf("mapped invoice was not reconciled to canonical contract: %+v", d.Findings)
	}
	if d.Billing[0].CustomerID != "cus_external" || d.Findings[0].EvidenceInvoiceIDs[0] != "in_1" {
		t.Fatal("mapping rewrote external evidence or lost original invoice reference")
	}
}

func TestMappingAlsoWorksForIndependentUsageAndCredits(t *testing.T) {
	d := base()
	d.Usage[0].CustomerID = "usage_external"
	d.Credits = []Credit{{ID: "cr_1", CustomerID: "credit_external", Period: "2026-08", Metric: "calls", AmountCents: 100, Reason: "approved"}}
	d.Mappings = []CustomerMapping{
		{Source: "usage_csv", ExternalCustomerID: "usage_external", CustomerID: "c1"},
		{Source: "credits_csv", ExternalCustomerID: "credit_external", CustomerID: "c1"},
	}
	Scan(&d, timeAt("2026-10-08"), 7)
	if len(d.Findings) != 1 || d.Findings[0].PotentialCents != 400 || d.Findings[0].CreditedCents != 100 {
		t.Fatalf("usage or approved credits mapped incorrectly: %+v", d.Findings)
	}
}

func TestMappedEvidenceClosesBothSourcesMissingCheck(t *testing.T) {
	d := NewDatabase()
	d.Contracts = []Contract{contractWithMinimum("2026-09")}
	d.Usage = []Usage{{ID: "u1", CustomerID: "external", Period: "2026-09", Metric: "calls", Quantity: 1}}
	Scan(&d, timeAt("2026-10-08"), 7)
	if len(d.Coverage) != 1 {
		t.Fatal("unmapped usage must not count as evidence for another customer")
	}
	d.Mappings = []CustomerMapping{{Source: "usage_csv", ExternalCustomerID: "external", CustomerID: "c1"}}
	Scan(&d, timeAt("2026-10-08"), 7)
	if len(d.Coverage) != 0 {
		t.Fatal("mapped usage should satisfy the both-sources-missing check")
	}
}

func TestMappingCSVValidationAndIdempotentUpsert(t *testing.T) {
	csv := "source,external_customer_id,customer_id\nstripe,cus_external,c1\nusage_csv,usage_id,c1\n"
	rows, err := ReadCSV("mappings", strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	d := NewDatabase()
	a := Import(&d, "mappings", rows)
	b := Import(&d, "mappings", rows)
	if a.Added != 2 || b.Updated != 2 || len(d.Mappings) != 2 {
		t.Fatalf("mapping import is not idempotent: %+v %+v %+v", a, b, d.Mappings)
	}
	invalid := []string{
		"source,external_customer_id,customer_id\nunknown,external,c1\n",
		"source,external_customer_id,customer_id\nstripe,external,c1\nstripe,external,c2\n",
		"source,external_customer_id,customer_id\nstripe,,c1\n",
	}
	for _, input := range invalid {
		if _, err := ReadCSV("mappings", strings.NewReader(input)); err == nil {
			t.Errorf("accepted invalid customer map: %s", input)
		}
	}
}

func TestMappingSnapshotsSurviveLaterChanges(t *testing.T) {
	d := base()
	d.Billing[0].Source = "stripe"
	d.Billing[0].CustomerID = "cus_external"
	d.Mappings = []CustomerMapping{{Source: "stripe", ExternalCustomerID: "cus_external", CustomerID: "c1"}}
	RunScan(&d, timeAt("2026-10-08"), 7, "import.mappings")
	first := d.ScanRuns[0].Sources.Mappings[0]
	d.Mappings[0].CustomerID = "another"
	RunScan(&d, timeAt("2026-10-09"), 7, "import.mappings")
	if d.ScanRuns[1].Sources.Mappings[0] != first || d.ScanRuns[0].SourceDigest == d.ScanRuns[1].SourceDigest {
		t.Fatal("mapping evidence history was mutated or fingerprint not updated")
	}
}

func TestDemoBillingSourceCannotBeAliasedAsCSV(t *testing.T) {
	d := base()
	d.Billing[0].Source = "demo"
	d.Billing[0].CustomerID = "demo_external"
	d.Mappings = []CustomerMapping{{Source: "billing_csv", ExternalCustomerID: "demo_external", CustomerID: "c1"}}
	Scan(&d, timeAt("2026-10-08"), 7)
	if d.Findings[0].PotentialCents != 2000 {
		t.Fatal("CSV map must not reassign a different provider's invoice")
	}
}
