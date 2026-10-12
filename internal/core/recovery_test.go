package core

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func recoveryFixture(t *testing.T) (Database, Finding) {
	t.Helper()
	d := base()
	RunScan(&d, timeAt("2026-10-08"), 7, "test")
	if len(d.Findings) != 1 {
		t.Fatalf("expected finding: %+v", d.Findings)
	}
	d.Findings[0].Status = "approved"
	return d, d.Findings[0]
}

func recoveryInput(f Finding, key, action string, amount int64) RecoveryInput {
	return RecoveryInput{IdempotencyKey: key, EvidenceDigest: f.EvidenceDigest, Action: action, AmountCents: amount, Note: "Reviewed contract and payment evidence"}
}

func TestRecoveryAssertionsAreIdempotentAndBounded(t *testing.T) {
	d, f := recoveryFixture(t)
	now := timeAt("2026-10-09")
	confirmed, err := RecordRecovery(&d, f.ID, recoveryInput(f, "confirm-case-001", "confirmed", 350), now)
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.AmountCents != 350 || confirmed.Currency != "USD" || len(d.Recoveries) != 1 {
		t.Fatalf("bad confirmation: %+v", d.Recoveries)
	}
	repeat, err := RecordRecovery(&d, f.ID, recoveryInput(f, "confirm-case-001", "confirmed", 350), now)
	if err != nil || repeat.ID != confirmed.ID || len(d.Recoveries) != 1 {
		t.Fatalf("retry doubled confirmation: %+v %v", repeat, err)
	}
	if _, err = RecordRecovery(&d, f.ID, recoveryInput(f, "confirm-case-002", "confirmed", 200), now); !errors.Is(err, ErrRecoveryInvalid) {
		t.Fatalf("confirmation exceeded finding: %v", err)
	}
	if _, err = RecordRecovery(&d, f.ID, recoveryInput(f, "collect-case-001", "collected", 250), now); err != nil {
		t.Fatal(err)
	}
	if _, err = RecordRecovery(&d, f.ID, recoveryInput(f, "collect-case-002", "collected", 101), now); !errors.Is(err, ErrRecoveryInvalid) {
		t.Fatalf("collection exceeded confirmed funds: %v", err)
	}
	if _, err = RecordRecovery(&d, f.ID, recoveryInput(f, "confirm-case-001", "confirmed", 100), now); !errors.Is(err, ErrRecoveryConflict) {
		t.Fatalf("idempotency key accepted different payload: %v", err)
	}
	totals := RecoveryTotals(d)["USD"]
	if totals.ConfirmedCurrentCents != 350 || totals.CollectedRecordedCents != 250 {
		t.Fatalf("conflated potential and recorded cash: %+v", totals)
	}
	if len(d.Audit) != 2 {
		t.Fatalf("expected one audit log per new event: %+v", d.Audit)
	}
}

func TestRecoveryRequiresFreshApprovedEvidence(t *testing.T) {
	d, f := recoveryFixture(t)
	d.Findings[0].Status = "reviewing"
	if _, err := RecordRecovery(&d, f.ID, recoveryInput(f, "confirm-case-003", "confirmed", 100), time.Now()); !errors.Is(err, ErrRecoveryInvalid) {
		t.Fatalf("unapproved finding accepted: %v", err)
	}
	d.Findings[0].Status = "approved"
	stale := recoveryInput(f, "confirm-case-004", "confirmed", 100)
	stale.EvidenceDigest = strings.Repeat("0", 64)
	if _, err := RecordRecovery(&d, f.ID, stale, time.Now()); !errors.Is(err, ErrRecoveryConflict) {
		t.Fatalf("stale evidence accepted: %v", err)
	}
	if _, err := RecordRecovery(&d, "missing-finding", recoveryInput(f, "confirm-case-005", "confirmed", 100), time.Now()); !errors.Is(err, ErrRecoveryNotFound) {
		t.Fatalf("missing finding accepted: %v", err)
	}
	if _, err := RecordRecovery(&d, f.ID, recoveryInput(f, "short", "confirmed", 100), time.Now()); !errors.Is(err, ErrRecoveryInvalid) {
		t.Fatalf("short idempotency key accepted: %v", err)
	}
	in := recoveryInput(f, "confirm-case-006", "confirmed", 100)
	in.Note = "no"
	if _, err := RecordRecovery(&d, f.ID, in, time.Now()); !errors.Is(err, ErrRecoveryInvalid) {
		t.Fatalf("unexplained finance assertion accepted: %v", err)
	}
	if len(d.Recoveries) != 0 {
		t.Fatal("failed assertions persisted")
	}
}

func TestRevokedCaseApprovalCannotCountCurrentConfirmation(t *testing.T) {
	d, f := recoveryFixture(t)
	if _, err := RecordRecovery(&d, f.ID, recoveryInput(f, "revoke-case-0001", "confirmed", 200), timeAt("2026-10-09")); err != nil {
		t.Fatal(err)
	}
	d.Findings[0].Status = "false_positive"
	totals := RecoveryTotals(d)["USD"]
	if totals.ConfirmedCurrentCents != 0 || totals.NeedsReviewCount != 1 {
		t.Fatalf("revoked review still treated as current confirmation: %+v", totals)
	}
}

func TestRecoveryEvidenceRevisionExpiresConfirmationButNotRecordedCollection(t *testing.T) {
	d, f := recoveryFixture(t)
	now := timeAt("2026-10-09")
	if _, err := RecordRecovery(&d, f.ID, recoveryInput(f, "confirm-case-007", "confirmed", 400), now); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordRecovery(&d, f.ID, recoveryInput(f, "collect-case-007", "collected", 200), now); err != nil {
		t.Fatal(err)
	}
	// A changed billing input changes the immutable evidence digest and
	// invalidates the review. Historical operator collection records remain.
	d.Billing[0].AmountCents = 1600
	Scan(&d, timeAt("2026-10-10"), 7)
	if d.Findings[0].Status != "new" {
		t.Fatalf("case was not reopened on new evidence: %+v", d.Findings[0])
	}
	totals := RecoveryTotals(d)["USD"]
	if totals.ConfirmedCurrentCents != 0 || totals.CollectedRecordedCents != 200 || totals.NeedsReviewCount != 2 {
		t.Fatalf("stale confirmation not separated from recorded historical collection: %+v", totals)
	}
	if _, err := RecordRecovery(&d, d.Findings[0].ID, recoveryInput(f, "confirm-case-008", "confirmed", 10), now); !errors.Is(err, ErrRecoveryConflict) {
		t.Fatalf("old fingerprint accepted for new evidence: %v", err)
	}
	if len(d.Recoveries) != 2 {
		t.Fatal("new evidence replaced historic recovery events")
	}
}

func TestRecoverySurvivesJSONRestart(t *testing.T) {
	path := t.TempDir() + "/workspace.json"
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	var entry RecoveryEntry
	if err := store.Update(func(d *Database) error {
		*d = base()
		RunScan(d, timeAt("2026-10-08"), 7, "migration")
		d.Findings[0].Status = "approved"
		var e error
		entry, e = RecordRecovery(d, d.Findings[0].ID, recoveryInput(d.Findings[0], "persist-case-0001", "confirmed", 250), timeAt("2026-10-09"))
		return e
	}); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := reopened.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Recoveries) != 1 || saved.Recoveries[0].ID != entry.ID || saved.Recoveries[0].AmountCents != 250 {
		t.Fatalf("lost recovery record on restart: %+v", saved.Recoveries)
	}
}
