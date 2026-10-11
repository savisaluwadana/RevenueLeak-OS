package core

import (
	"context"
	"testing"
	"time"
)

func TestScheduledScanAvoidsDuplicateWritesAndPreservesEvidence(t *testing.T) {
	store, err := Open(t.TempDir() + "/store.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update(func(d *Database) error {
		*d = base()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	now := timeAt("2026-10-08")
	ran, err := ScheduledScan(store, now, 7)
	if err != nil || !ran {
		t.Fatalf("first scan did not run: %v", err)
	}
	before := store.Snapshot()
	if len(before.ScanRuns) != 1 || before.ScanRuns[0].Trigger != "scheduled" {
		t.Fatalf("first run was not saved: %+v", before.ScanRuns)
	}
	ran, err = ScheduledScan(store, now, 7)
	if err != nil || ran {
		t.Fatalf("duplicate daily scan was not skipped: %v", err)
	}
	after := store.Snapshot()
	if len(after.ScanRuns) != 1 || after.ScanSequence != before.ScanSequence || after.UpdatedAt != before.UpdatedAt {
		t.Fatal("identical scheduled scan wrote another snapshot")
	}
	if err := store.Update(func(d *Database) error {
		d.Usage[0].Quantity += 100
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	ran, err = ScheduledScan(store, now, 7)
	if err != nil || !ran {
		t.Fatalf("changed sources must trigger same-day scan: %v", err)
	}
	if len(store.Snapshot().ScanRuns) != 2 {
		t.Fatal("changed source scan not recorded")
	}
	ran, err = ScheduledScan(store, now.AddDate(0, 0, 1), 7)
	if err != nil || !ran || len(store.Snapshot().ScanRuns) != 3 {
		t.Fatalf("new UTC day must trigger re-evaluation: %v", err)
	}
}

func TestScheduledScanAccountsForCustomerMappingChanges(t *testing.T) {
	store, err := Open(t.TempDir() + "/store.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update(func(d *Database) error {
		*d = base()
		d.Mappings = []CustomerMapping{{Source: "stripe", ExternalCustomerID: "cus_old", CustomerID: "c1"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	now := timeAt("2026-10-08")
	if ran, err := ScheduledScan(store, now, 7); err != nil || !ran {
		t.Fatalf("initial scan failed: ran=%v err=%v", ran, err)
	}
	if ran, err := ScheduledScan(store, now, 7); err != nil || ran {
		t.Fatalf("unchanged mapped sources must not re-run: ran=%v err=%v", ran, err)
	}
	if err := store.Update(func(d *Database) error {
		d.Mappings[0].CustomerID = "c2"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if ran, err := ScheduledScan(store, now, 7); err != nil || !ran {
		t.Fatalf("mapping changes must re-run the audit: ran=%v err=%v", ran, err)
	}
	if got := len(store.Snapshot().ScanRuns); got != 2 {
		t.Fatalf("expected two evidence runs, got %d", got)
	}
}

func TestScheduledScanRechecksRuleVersion(t *testing.T) {
	store, err := Open(t.TempDir() + "/store.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update(func(d *Database) error {
		*d = base()
		RunScan(d, timeAt("2026-10-08"), 7, "manual")
		d.ScanRuns[0].RuleVersion = "outdated-rules"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	ran, err := ScheduledScan(store, timeAt("2026-10-08"), 7)
	if err != nil || !ran {
		t.Fatalf("rule change must trigger a scan: %v", err)
	}
}

func TestScheduledLoopRespectsCancellation(t *testing.T) {
	store, err := Open(t.TempDir() + "/store.json")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		RunScheduledScans(ctx, store, time.Hour, 7, func(e error) {
			t.Errorf("scheduled scan failed: %v", e)
		})
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler did not stop on cancellation")
	}
}
