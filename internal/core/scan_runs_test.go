package core

import (
	"encoding/json"
	"os"
	"testing"
)

func TestSavedRunOwnsInputsAndSurvivesFindingDisappearance(t *testing.T) {
	d := base()
	summary := RunScan(&d, timeAt("2026-10-08"), 7, "manual")
	original, _ := json.Marshal(d.ScanRuns[0])
	if summary.RunID == "" || d.ScanRuns[0].SourceDigest == "" {
		t.Fatal("missing run identity")
	}
	d.Contracts[0].CustomerName = "Changed"
	d.Usage[0].Quantity = 100
	d.Findings[0].Note = "edited after scan"
	d.Findings[0].EvidenceUsageIDs[0] = "changed"
	RunScan(&d, timeAt("2026-10-09"), 7, "import.usage")
	if len(d.Findings) != 0 || len(d.ScanRuns) != 2 {
		t.Fatalf("unexpected current/history sizes: %d/%d", len(d.Findings), len(d.ScanRuns))
	}
	saved, _ := json.Marshal(d.ScanRuns[1])
	if string(original) != string(saved) {
		t.Fatal("saved run mutated with live inputs or findings")
	}
}

func TestReviewedEvidenceChangesReopenFinding(t *testing.T) {
	for _, status := range []string{"reviewing", "approved", "resolved", "false_positive"} {
		t.Run(status, func(t *testing.T) {
			d := base()
			RunScan(&d, timeAt("2026-10-08"), 7, "manual")
			d.Findings[0].Status = status
			d.Findings[0].Note = "Earlier review"
			first := d.Findings[0].FirstSeen
			d.Usage[0].Quantity += 50
			s := RunScan(&d, timeAt("2026-10-09"), 7, "import.usage")
			f := d.Findings[0]
			if f.Status != "new" || f.Note != "Earlier review" || f.FirstSeen != first || f.ReviewRequiredReason == "" || s.ReopenedFindings != 1 {
				t.Fatalf("stale review was retained: %+v", f)
			}
		})
	}
}

func TestEvidenceFingerprintIgnoresInputRowOrder(t *testing.T) {
	d := base()
	d.Usage = append(d.Usage, Usage{ID: "u2", CustomerID: "c1", Period: "2026-08", Metric: "calls", Quantity: 10})
	RunScan(&d, timeAt("2026-10-08"), 7, "manual")
	d.Findings[0].Status = "approved"
	d.Usage[0], d.Usage[1] = d.Usage[1], d.Usage[0]
	s := RunScan(&d, timeAt("2026-10-09"), 7, "import.usage")
	if d.Findings[0].Status != "approved" || s.ReopenedFindings != 0 {
		t.Fatal("row order reopened a reviewed case")
	}
}

func TestRetentionKeepsUniqueRunIDsAndLatestRuns(t *testing.T) {
	d := base()
	for i := 0; i < MaxSavedScans+3; i++ {
		RunScan(&d, timeAt("2026-10-08"), 7, "manual")
	}
	if len(d.ScanRuns) != MaxSavedScans || d.ScanRuns[0].ID != "scan_00000028" || d.ScanRuns[MaxSavedScans-1].ID != "scan_00000004" {
		t.Fatalf("wrong retention: %d runs", len(d.ScanRuns))
	}
}

func TestRunHistoryPersistsAndUpgradesLegacyDatastore(t *testing.T) {
	path := t.TempDir() + "/legacy.json"
	if err := os.WriteFile(path, []byte(`{"contracts":[],"usage":[],"billing":[],"credits":[],"findings":[],"audit":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Update(func(d *Database) error { RunScan(d, timeAt("2026-10-08"), 7, "manual"); return nil }); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := reopened.Snapshot()
	if len(snapshot.ScanRuns) != 1 || snapshot.ScanSequence != 1 {
		t.Fatal("history lost after restart")
	}
	if err = reopened.Update(func(d *Database) error { RunScan(d, timeAt("2026-10-09"), 7, "manual"); return errSentinel{} }); err == nil {
		t.Fatal("expected rollback")
	}
	if reopened.Snapshot().ScanSequence != 1 || len(reopened.Snapshot().ScanRuns) != 1 {
		t.Fatal("failed transaction changed history")
	}
}
