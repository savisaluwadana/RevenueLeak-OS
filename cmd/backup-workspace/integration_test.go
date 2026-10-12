//go:build postgres_integration

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"revenueleak-os/internal/core"
	"revenueleak-os/internal/pgstore"
)

func TestBackedUpWorkspaceCanRestoreIntoFreshOrganization(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_URL")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_URL missing")
	}
	source, err := pgstore.Open(dsn, "ffffffff-ffff-4fff-8fff-ffffffffffff")
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	fresh, err := pgstore.Open(dsn, "99999999-9999-4999-8999-999999999999")
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()

	if err := source.Update(func(d *core.Database) error {
		d.Contracts = []core.Contract{{CustomerID: "c1", CustomerName: "Backed up customer", Metric: "tokens", Currency: "USD", UnitPriceCents: 25, EffectiveFrom: "2026-09"}}
		d.Findings = []core.Finding{{ID: "case-1", CustomerID: "c1", Status: "reviewing", Note: "retain original review notes"}}
		d.ScanSequence = 8
		d.ScanRuns = []core.ScanRun{{ID: "scan_00000008", Trigger: "manual", RuleVersion: core.RuleVersion}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	original, err := source.Read()
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "workspace-backup.json")
	want, err := writeBackup(out, original)
	if err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var recovered core.Database
	if err := json.Unmarshal(contents, &recovered); err != nil {
		t.Fatal(err)
	}
	if err := fresh.ImportEmptyWorkspace(recovered); err != nil {
		t.Fatalf("restore import failed: %v", err)
	}
	restored, err := fresh.Read()
	if err != nil {
		t.Fatal(err)
	}
	got, err := fingerprint(restored)
	if err != nil {
		t.Fatal(err)
	}
	if got != want || restored.Findings[0].Note != "retain original review notes" || restored.ScanRuns[0].ID != "scan_00000008" {
		t.Fatalf("restore evidence differs from backup: digest %s expected %s", got, want)
	}
}
