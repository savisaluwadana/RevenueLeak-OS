package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

const MaxSavedScans = 25
const RuleVersion = "monthly-metered-v1+coverage-v1"

// SourceSnapshot contains the precise inputs used for a reconciliation run.
// It excludes passwords, sessions and connector credentials.
type SourceSnapshot struct {
	Contracts []Contract `json:"contracts"`
	Usage     []Usage    `json:"usage"`
	Billing   []Billing  `json:"billing"`
	Credits   []Credit   `json:"credits"`
}

type ScanRun struct {
	ID           string         `json:"id"`
	RuleVersion  string         `json:"rule_version"`
	GraceDays    int            `json:"grace_days"`
	Trigger      string         `json:"trigger"`
	Summary      ScanSummary    `json:"summary"`
	SourceDigest string         `json:"source_digest"`
	Sources      SourceSnapshot `json:"sources"`
	Findings     []Finding      `json:"findings"`
	Coverage     []CoverageGap  `json:"coverage"`
}

type ScanRunIndex struct {
	ID           string      `json:"id"`
	RuleVersion  string      `json:"rule_version"`
	GraceDays    int         `json:"grace_days"`
	Trigger      string      `json:"trigger"`
	Summary      ScanSummary `json:"summary"`
	SourceDigest string      `json:"source_digest"`
}

func digest(value any) string {
	// These types contain only strings, integers and slices: JSON encoding cannot fail.
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func findingEvidenceDigest(contract Contract, usage []Usage, billing []Billing, credits []Credit) string {
	usage = append([]Usage{}, usage...)
	billing = append([]Billing{}, billing...)
	credits = append([]Credit{}, credits...)
	sort.Slice(usage, func(i, j int) bool { return usage[i].ID < usage[j].ID })
	sort.Slice(billing, func(i, j int) bool { return billing[i].ID < billing[j].ID })
	sort.Slice(credits, func(i, j int) bool { return credits[i].ID < credits[j].ID })
	return digest(SourceSnapshot{Contracts: []Contract{contract}, Usage: usage, Billing: billing, Credits: credits})
}

// RunScan is called inside Store.Update so sources, findings and history commit
// together. Saved values own their slices and cannot change with a later import.
func RunScan(d *Database, now time.Time, graceDays int, trigger string) ScanSummary {
	summary := Scan(d, now, graceDays)
	d.ScanSequence++
	summary.RunID = fmt.Sprintf("scan_%08d", d.ScanSequence)
	sources := SourceSnapshot{
		Contracts: append([]Contract{}, d.Contracts...),
		Usage:     append([]Usage{}, d.Usage...),
		Billing:   append([]Billing{}, d.Billing...),
		Credits:   append([]Credit{}, d.Credits...),
	}
	findings := append([]Finding{}, d.Findings...)
	for i := range findings {
		findings[i].EvidenceUsageIDs = append([]string{}, findings[i].EvidenceUsageIDs...)
		findings[i].EvidenceInvoiceIDs = append([]string{}, findings[i].EvidenceInvoiceIDs...)
	}
	run := ScanRun{ID: summary.RunID, RuleVersion: RuleVersion, GraceDays: graceDays, Trigger: trigger,
		Summary: summary, SourceDigest: digest(sources), Sources: sources, Findings: findings,
		Coverage: append([]CoverageGap{}, d.Coverage...)}
	d.ScanRuns = append([]ScanRun{run}, d.ScanRuns...)
	if len(d.ScanRuns) > MaxSavedScans {
		d.ScanRuns = append([]ScanRun{}, d.ScanRuns[:MaxSavedScans]...)
	}
	d.LastScan = summary
	return summary
}
