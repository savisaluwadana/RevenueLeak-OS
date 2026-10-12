package core

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
)

// RecoveryEntry is an operator-entered assertion, not proof of receipt of
// funds. Monetary calculations never consume these entries as invoice inputs.
// EvidenceDigest links the assertion to the exact finding review version.
type RecoveryEntry struct {
	ID             string `json:"id"`
	FindingID      string `json:"finding_id"`
	CustomerID     string `json:"customer_id"`
	Currency       string `json:"currency"`
	EvidenceDigest string `json:"evidence_digest"`
	Action         string `json:"action"`
	AmountCents    int64  `json:"amount_cents"`
	Note           string `json:"note"`
	At             string `json:"at"`
}

type RecoveryInput struct {
	IdempotencyKey string `json:"idempotency_key"`
	EvidenceDigest string `json:"evidence_digest"`
	Action         string `json:"action"`
	AmountCents    int64  `json:"amount_cents"`
	Note           string `json:"note"`
}

var ErrRecoveryInvalid = errors.New("invalid recovery record")
var ErrRecoveryConflict = errors.New("stale evidence or idempotency conflict")
var ErrRecoveryNotFound = errors.New("finding not found")

var recoveryKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]{8,128}$`)

// RecordRecovery adds one append-only, bounded monetary review assertion.
// Idempotency keys prevent network retries from double-counting. A confirmation
// requires a reviewed finding; collection requires previously confirmed funds
// linked to the SAME evidence. Evidence changes block further recording until
// finance has reviewed the new finding and reconfirmed the amount.
func RecordRecovery(d *Database, findingID string, in RecoveryInput, now time.Time) (RecoveryEntry, error) {
	if !recoveryKeyPattern.MatchString(in.IdempotencyKey) || (in.Action != "confirmed" && in.Action != "collected") ||
		in.AmountCents <= 0 || len(strings.TrimSpace(in.Note)) < 8 || len(in.Note) > 1500 ||
		len(in.EvidenceDigest) != 64 {
		return RecoveryEntry{}, fmt.Errorf("%w: valid key, evidence digest, positive amount and explanatory note are required", ErrRecoveryInvalid)
	}
	// Stable across retries of the same request but not the same key for
	// another case. A digest ID avoids displaying the caller's request key.
	keySum := sha256.Sum256([]byte(findingID + ":" + in.IdempotencyKey))
	id := "rec_" + hex.EncodeToString(keySum[:16])
	for _, event := range d.Recoveries {
		if event.ID == id {
			if event.FindingID == findingID && event.EvidenceDigest == in.EvidenceDigest &&
				event.Action == in.Action && event.AmountCents == in.AmountCents && event.Note == strings.TrimSpace(in.Note) {
				return event, nil
			}
			return RecoveryEntry{}, fmt.Errorf("%w: idempotency key was already used for a different operation", ErrRecoveryConflict)
		}
	}
	var finding *Finding
	for i := range d.Findings {
		if d.Findings[i].ID == findingID {
			finding = &d.Findings[i]
			break
		}
	}
	if finding == nil {
		return RecoveryEntry{}, ErrRecoveryNotFound
	}
	if finding.EvidenceDigest == "" || finding.EvidenceDigest != in.EvidenceDigest {
		return RecoveryEntry{}, fmt.Errorf("%w: reload this investigation and verify the latest source evidence", ErrRecoveryConflict)
	}
	if finding.Status != "approved" && finding.Status != "resolved" {
		return RecoveryEntry{}, fmt.Errorf("%w: case must first be approved by the workspace reviewer", ErrRecoveryInvalid)
	}
	var confirmed, collected int64
	for _, event := range d.Recoveries {
		if event.FindingID != findingID || event.EvidenceDigest != in.EvidenceDigest {
			continue
		}
		var candidate int64
		var ok bool
		if event.Action == "confirmed" {
			candidate, ok = safeSum(confirmed, event.AmountCents)
			if !ok {
				return RecoveryEntry{}, fmt.Errorf("%w: confirmed total overflow", ErrRecoveryInvalid)
			}
			confirmed = candidate
		} else if event.Action == "collected" {
			candidate, ok = safeSum(collected, event.AmountCents)
			if !ok {
				return RecoveryEntry{}, fmt.Errorf("%w: collected total overflow", ErrRecoveryInvalid)
			}
			collected = candidate
		}
	}
	if in.Action == "confirmed" {
		if confirmed > finding.PotentialCents || in.AmountCents > finding.PotentialCents-confirmed {
			return RecoveryEntry{}, fmt.Errorf("%w: cumulative confirmation exceeds the potential finding for this evidence", ErrRecoveryInvalid)
		}
	} else {
		if collected > confirmed || in.AmountCents > confirmed-collected {
			return RecoveryEntry{}, fmt.Errorf("%w: recorded collection exceeds confirmed amount for this evidence", ErrRecoveryInvalid)
		}
	}
	if len(d.Recoveries) >= 10000 {
		return RecoveryEntry{}, fmt.Errorf("%w: pilot record limit reached; export and upgrade the datastore", ErrRecoveryInvalid)
	}
	event := RecoveryEntry{
		ID: id, FindingID: findingID, CustomerID: finding.CustomerID,
		Currency: finding.Currency, EvidenceDigest: in.EvidenceDigest,
		Action: in.Action, AmountCents: in.AmountCents,
		Note: strings.TrimSpace(in.Note), At: now.UTC().Format(time.RFC3339),
	}
	d.Recoveries = append(d.Recoveries, event)
	Log(d, "recovery."+in.Action, fmt.Sprintf("%s %d %s minor units (operator assertion only)", findingID, in.AmountCents, finding.Currency))
	return event, nil
}

type RecoveryCurrencyTotals struct {
	ConfirmedCurrentCents  int64 `json:"confirmed_current_cents"`
	CollectedRecordedCents int64 `json:"collected_recorded_cents"`
	NeedsReviewCount       int   `json:"needs_review_count"`
}

// RecoveryTotals returns separately labeled operator assertions; it does not
// certify cash collection. Historical collected entries are shown even after
// their finding disappears or changes, while stale confirmations are excluded
// from the current evidence total.
func RecoveryTotals(d Database) map[string]RecoveryCurrencyTotals {
	findings := make(map[string]Finding, len(d.Findings))
	for _, f := range d.Findings {
		findings[f.ID] = f
	}
	out := map[string]RecoveryCurrencyTotals{}
	for _, event := range d.Recoveries {
		t := out[event.Currency]
		f, ok := findings[event.FindingID]
		current := ok && f.EvidenceDigest == event.EvidenceDigest && (f.Status == "approved" || f.Status == "resolved")
		switch event.Action {
		case "confirmed":
			if current && event.AmountCents > 0 && t.ConfirmedCurrentCents <= math.MaxInt64-event.AmountCents {
				t.ConfirmedCurrentCents += event.AmountCents
			} else if !current {
				t.NeedsReviewCount++
			}
		case "collected":
			if event.AmountCents > 0 && t.CollectedRecordedCents <= math.MaxInt64-event.AmountCents {
				t.CollectedRecordedCents += event.AmountCents
			}
			if !current {
				t.NeedsReviewCount++
			}
		}
		out[event.Currency] = t
	}
	return out
}
