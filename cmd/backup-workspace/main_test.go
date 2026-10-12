package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"revenueleak-os/internal/core"
)

func exampleSnapshot() core.Database {
	d := core.NewDatabase()
	d.Contracts = []core.Contract{{CustomerID: "c1", CustomerName: "Acme", Metric: "tokens", Currency: "USD", UnitPriceCents: 10, EffectiveFrom: "2026-01"}}
	d.Billing = []core.Billing{{ID: "line-1", InvoiceID: "inv-1", CustomerID: "c1", Period: "2026-09", Metric: "tokens", Currency: "USD", AmountCents: 900, Status: "paid", Source: "billing_csv"}}
	d.Findings = []core.Finding{{ID: "f1", CustomerID: "c1", Status: "reviewing", Note: "finance evidence preserved"}}
	d.ScanSequence = 2
	d.ScanRuns = []core.ScanRun{{ID: "scan_00000002", RuleVersion: core.RuleVersion}}
	d.UpdatedAt = "2026-10-11T12:00:00Z"
	return d
}

func TestBackupWritesRestrictedVerifiedSnapshot(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "finance-backup.json")
	original := exampleSnapshot()
	expected, err := fingerprint(original)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := writeBackup(dest, original)
	if err != nil {
		t.Fatal(err)
	}
	if digest != expected {
		t.Fatalf("backup digest mismatch: %s %s", digest, expected)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("sensitive backup is not 0600: %v", info.Mode().Perm())
	}
	raw, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	var recovered core.Database
	if err := json.Unmarshal(raw, &recovered); err != nil {
		t.Fatal(err)
	}
	if recovered.ScanSequence != 2 || recovered.ScanRuns[0].ID != original.ScanRuns[0].ID ||
		recovered.Findings[0].Note != original.Findings[0].Note ||
		recovered.Billing[0].InvoiceID != original.Billing[0].InvoiceID {
		t.Fatalf("backup lost audit/financial fields: %+v", recovered)
	}
	actual, err := fingerprint(recovered)
	if err != nil || actual != expected {
		t.Fatalf("backup data mismatch: %s / %s / %v", actual, expected, err)
	}
	if matches, err := filepath.Glob(filepath.Join(filepath.Dir(dest), ".revenueleak-backup-*")); err != nil || len(matches) != 0 {
		t.Fatalf("temporary backup files remain: %v %v", matches, err)
	}
}

func TestBackupNeverOverwritesExistingFileOrSymlink(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "existing.json")
	old := []byte("ORIGINAL FINANCE BACKUP")
	if err := os.WriteFile(p, old, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := writeBackup(p, exampleSnapshot()); err == nil {
		t.Fatal("existing financial backup was overwritten")
	}
	unchanged, err := os.ReadFile(p)
	if err != nil || string(unchanged) != string(old) {
		t.Fatalf("existing backup changed: %q %v", unchanged, err)
	}
	link := filepath.Join(dir, "symlink.json")
	if err := os.Symlink(p, link); err != nil {
		t.Fatal(err)
	}
	if _, err := writeBackup(link, exampleSnapshot()); err == nil {
		t.Fatal("symlink destination accepted")
	}
	unchanged, err = os.ReadFile(p)
	if err != nil || string(unchanged) != string(old) {
		t.Fatalf("symlink target changed: %q %v", unchanged, err)
	}
}

func TestBackupRejectsMissingDestination(t *testing.T) {
	if _, err := writeBackup("", exampleSnapshot()); err == nil {
		t.Fatal("missing backup destination accepted")
	}
	if _, err := writeBackup(filepath.Join(t.TempDir(), "missing", "backup.json"), exampleSnapshot()); err == nil {
		t.Fatal("unknown backup directory was silently created")
	}
}

func TestBackupDigestChangesWhenFinancialEvidenceChanges(t *testing.T) {
	a := exampleSnapshot()
	b := exampleSnapshot()
	b.Findings[0].Note = "approved different evidence"
	ha, err := fingerprint(a)
	if err != nil {
		t.Fatal(err)
	}
	hb, err := fingerprint(b)
	if err != nil {
		t.Fatal(err)
	}
	if ha == hb || len(ha) != 64 || !strings.Contains(a.Findings[0].Note, "preserved") {
		t.Fatal("evidence mutation did not change SHA-256")
	}
}
