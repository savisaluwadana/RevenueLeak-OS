package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"revenueleak-os/internal/core"
)

func fixture(t *testing.T) string {
	t.Helper()
	d := core.NewDatabase()
	d.Contracts = []core.Contract{{CustomerID: "c1", CustomerName: "Acme", Metric: "tokens", Currency: "USD", UnitPriceCents: 9, EffectiveFrom: "2026-01"}}
	d.ScanSequence = 5
	d.ScanRuns = []core.ScanRun{{ID: "scan_00000005", Trigger: "manual", RuleVersion: core.RuleVersion}}
	d.LastScan = core.ScanSummary{RunID: "scan_00000005", Findings: 2}
	d.UpdatedAt = "2026-10-11T10:00:00Z"
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "pilot.json")
	if err = os.WriteFile(p, data, 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadSnapshotPreservesEvidenceAndCanonicalHash(t *testing.T) {
	path := fixture(t)
	got, err := loadSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Contracts) != 1 || got.ScanSequence != 5 || len(got.ScanRuns) != 1 || got.LastScan.RunID != "scan_00000005" || got.UpdatedAt != "2026-10-11T10:00:00Z" {
		t.Fatalf("pilot financial and historical state was changed: %+v", got)
	}
	digest, err := snapshotDigest(got)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var canonical core.Database
	if err := json.Unmarshal(data, &canonical); err != nil {
		t.Fatal(err)
	}
	canonicalHash, err := snapshotDigest(canonical)
	if err != nil || canonicalHash != digest {
		t.Fatalf("canonical JSON hash changed: %s %s %v", digest, canonicalHash, err)
	}
}

func TestLoadSnapshotRejectsUnsafeOrInvalidSources(t *testing.T) {
	dir := t.TempDir()
	ok := fixture(t)
	bad := []struct {
		name, content string
	}{
		{"empty", ""},
		{"null", "null"},
		{"trailing", "{}{}"},
		{"unknown", `{"future_unknown_field":1}`},
		{"corrupt", "{invalid-json"},
		{"negative-sequence", `{"scan_sequence":-2}`},
		{"duplicate-runs", `{"scan_runs":[{"id":"scan_1"},{"id":"scan_1"}]}`},
		{"missing-id", `{"scan_runs":[{"id":""}]}`},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(dir, tc.name+".json")
			if err := os.WriteFile(p, []byte(tc.content), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadSnapshot(p); err == nil {
				t.Fatal("invalid financial snapshot accepted")
			}
		})
	}
	link := filepath.Join(dir, "source-link.json")
	if err := os.Symlink(ok, link); err != nil {
		t.Fatal(err)
	}
	if _, err := loadSnapshot(link); err == nil || !strings.Contains(err.Error(), "non-symlink") {
		t.Fatalf("symlink was accepted: %v", err)
	}
	if _, err := loadSnapshot(dir); err == nil {
		t.Fatal("directory was accepted")
	}
	if _, err := loadSnapshot(""); err == nil {
		t.Fatal("missing source was accepted")
	}
}

func TestSnapshotRejectsRetentionBeyondExistingLimit(t *testing.T) {
	d := core.NewDatabase()
	for i := 0; i < core.MaxSavedScans+1; i++ {
		d.ScanRuns = append(d.ScanRuns, core.ScanRun{ID: "run_"+strings.Repeat("a", i+1)})
	}
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "oversized-history.json")
	if err := os.WriteFile(p, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadSnapshot(p); err == nil {
		t.Fatal("snapshot exceeded documented history retention")
	}
}
