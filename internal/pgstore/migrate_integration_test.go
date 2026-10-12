//go:build postgres_integration

package pgstore

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"revenueleak-os/internal/core"
)

const migratedOrgID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
const initializedOrgID = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"

func TestImportEmptyWorkspacePreservesCompleteSnapshotAndRejectsOverwrite(t *testing.T) {
	s := openTestStore(t, migratedOrgID)
	original := core.NewDatabase()
	original.Contracts = []core.Contract{{CustomerID: "c1", CustomerName: "Acme", Metric: "tokens", Currency: "USD", UnitPriceCents: 10, EffectiveFrom: "2026-01"}}
	original.Mappings = []core.CustomerMapping{{Source: "stripe", ExternalCustomerID: "cus_external", CustomerID: "c1"}}
	original.Usage = []core.Usage{{ID: "u1", CustomerID: "c1", Period: "2026-08", Metric: "tokens", Quantity: 200}}
	original.Billing = []core.Billing{{ID: "b1", InvoiceID: "inv1", CustomerID: "c1", Period: "2026-08", Metric: "tokens", Currency: "USD", AmountCents: 100, Status: "paid", Source: "stripe"}}
	original.Findings = []core.Finding{{ID: "leak_1", CustomerID: "c1", CustomerName: "Acme", Metric: "tokens", Period: "2026-08", Status: "reviewing", PotentialCents: 1900, Note: "finance investigating"}}
	original.ScanSequence = 17
	original.ScanRuns = []core.ScanRun{{ID: "scan_00000017", RuleVersion: core.RuleVersion, SourceDigest: "fake-source-digest-for-fixture", Trigger: "manual", Summary: core.ScanSummary{RunID: "scan_00000017", Findings: 1}}}
	original.LastScan = core.ScanSummary{RunID: "scan_00000017", Findings: 1}
	original.UpdatedAt = "2026-09-24T15:16:17Z"

	if err := s.ImportEmptyWorkspace(original); err != nil {
		t.Fatalf("import failed: %v", err)
	}
	reopened := openTestStore(t, migratedOrgID)
	got, err := reopened.Read()
	if err != nil {
		t.Fatal(err)
	}
	a, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("migration changed financial evidence; source=%s target=%s", a, b)
	}
	different := core.NewDatabase()
	different.Contracts = []core.Contract{{CustomerID: "other", CustomerName: "Overwritten"}}
	if err := s.ImportEmptyWorkspace(different); !errors.Is(err, ErrWorkspaceNotEmpty) {
		t.Fatalf("import overwrote initialized target or wrong error: %v", err)
	}
	again, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	c, err := json.Marshal(again)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, c) {
		t.Fatal("rejected overwrite nevertheless changed the target")
	}
}

func TestImportRefusesPreviouslyInitializedButEmptyWorkspace(t *testing.T) {
	s := openTestStore(t, initializedOrgID)
	if err := s.Update(func(d *core.Database) error {
		return nil // writes an initialized empty snapshot, increasing version.
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.ImportEmptyWorkspace(core.NewDatabase()); !errors.Is(err, ErrWorkspaceNotEmpty) {
		t.Fatalf("previously initialized workspace incorrectly accepted import: %v", err)
	}
}
