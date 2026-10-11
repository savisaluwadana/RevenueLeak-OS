package core

import (
	"context"
	"fmt"
	"time"
)

// scheduledScanDue compares both the source fingerprint and the UTC date.
// A new day can make another completed billing month eligible, while an
// importer can change evidence within the same day.
func scheduledScanDue(d *Database, now time.Time, graceDays int) bool {
	if len(d.ScanRuns) == 0 {
		return true
	}
	last := d.ScanRuns[0]
	date := now.UTC().Format("2006-01-02")
	if last.RuleVersion != RuleVersion || last.GraceDays != graceDays || len(last.Summary.At) < 10 || last.Summary.At[:10] != date {
		return true
	}
	inputs := SourceSnapshot{
		Contracts: d.Contracts,
		Mappings:  d.Mappings,
		Usage:     d.Usage,
		Billing:   d.Billing,
		Credits:   d.Credits,
	}
	return last.SourceDigest != digest(inputs)
}

// ScheduledScan checks whether a saved scan for the same current inputs and
// UTC day already exists. No write happens when the scan is not due.
// It must not be used as proof that external data sources are complete.
func ScheduledScan(store *Store, now time.Time, graceDays int) (bool, error) {
	snapshot := store.Snapshot()
	if !scheduledScanDue(&snapshot, now, graceDays) {
		return false, nil
	}
	var ran bool
	err := store.Update(func(d *Database) error {
		if !scheduledScanDue(d, now, graceDays) {
			return nil
		}
		s := RunScan(d, now, graceDays, "scheduled")
		Log(d, "scan.scheduled", fmt.Sprintf("%d monetary findings, %d data coverage gaps", s.Findings, s.CoverageGaps))
		ran = true
		return nil
	})
	return ran, err
}

// RunScheduledScans first checks at startup, then on each interval until the
// service context is cancelled. The caller configures a safe interval.
func RunScheduledScans(ctx context.Context, store *Store, interval time.Duration, graceDays int, onError func(error)) {
	check := func() {
		if _, err := ScheduledScan(store, time.Now(), graceDays); err != nil && onError != nil {
			onError(err)
		}
	}
	check()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			check()
		}
	}
}
