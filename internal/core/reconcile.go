package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type pair struct{ customer, period, metric string }

func (p pair) String() string { return p.customer + "|" + p.period + "|" + p.metric }
func periodEligible(period string, now time.Time, graceDays int) bool {
	t, err := time.Parse("2006-01", period)
	if err != nil {
		return false
	}
	return !now.Before(t.AddDate(0, 1, 0).AddDate(0, 0, graceDays))
}
func within(c Contract, period string) bool {
	return period >= c.EffectiveFrom && (c.EffectiveTo == "" || period <= c.EffectiveTo)
}
func contractFor(contracts []Contract, k pair) (Contract, bool) {
	var found Contract
	ok := false
	for _, c := range contracts {
		if c.CustomerID == k.customer && c.Metric == k.metric && within(c, k.period) && (!ok || c.EffectiveFrom > found.EffectiveFrom) {
			found = c
			ok = true
		}
	}
	return found, ok
}
func findingID(k pair) string {
	sum := sha256.Sum256([]byte(k.String()))
	return "leak_" + hex.EncodeToString(sum[:8])
}
func safeSum(a, b int64) (int64, bool) {
	if b > 0 && a > math.MaxInt64-b {
		return 0, false
	}
	if b < 0 && a < math.MinInt64-b {
		return 0, false
	}
	return a + b, true
}
func safeProduct(a, b int64) (int64, bool) {
	if a < 0 || b < 0 {
		return 0, false
	}
	if a > 0 && b > math.MaxInt64/a {
		return 0, false
	}
	return a * b, true
}

type aggregate struct {
	usage, billedUnits, billed, credits int64
	uIDs, invoiceIDs                    []string
	currencyMismatch, overflow          bool
}

func (a *aggregate) add(field *int64, amount int64) {
	v, ok := safeSum(*field, amount)
	if !ok {
		a.overflow = true
	} else {
		*field = v
	}
}
func Scan(d *Database, now time.Time, graceDays int) ScanSummary {
	byPair := map[pair]*aggregate{}
	get := func(k pair) *aggregate {
		a, ok := byPair[k]
		if !ok {
			a = &aggregate{uIDs: []string{}, invoiceIDs: []string{}}
			byPair[k] = a
		}
		return a
	}
	for _, u := range d.Usage {
		a := get(pair{u.CustomerID, u.Period, u.Metric})
		a.add(&a.usage, u.Quantity)
		a.uIDs = append(a.uIDs, u.ID)
	}
	for _, b := range d.Billing {
		if b.Status != "paid" && b.Status != "open" {
			continue
		}
		a := get(pair{b.CustomerID, b.Period, b.Metric})
		// Currency reconciliation is deliberately deferred until the effective contract is selected.
		a.invoiceIDs = append(a.invoiceIDs, b.InvoiceID)
	}
	// Avoid re-scanning entire source arrays for every candidate tuple.
	billingByPair := map[pair][]Billing{}
	for _, b := range d.Billing {
		if b.Status == "paid" || b.Status == "open" {
			k := pair{b.CustomerID, b.Period, b.Metric}
			billingByPair[k] = append(billingByPair[k], b)
		}
	}
	for _, cr := range d.Credits {
		k := pair{cr.CustomerID, cr.Period, cr.Metric}
		if a, ok := byPair[k]; ok {
			a.add(&a.credits, cr.AmountCents)
		}
	}
	keys := make([]pair, 0, len(byPair))
	for k := range byPair {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
	previous := map[string]Finding{}
	for _, f := range d.Findings {
		previous[f.ID] = f
	}
	summary := ScanSummary{At: now.UTC().Format(time.RFC3339)}
	findings := []Finding{}
	for _, k := range keys {
		if !periodEligible(k.period, now, graceDays) {
			summary.SkippedOpenPeriods++
			continue
		}
		c, ok := contractFor(d.Contracts, k)
		if !ok {
			summary.SkippedNoContract++
			continue
		}
		a := byPair[k]
		for _, b := range billingByPair[k] {
			if !strings.EqualFold(b.Currency, c.Currency) {
				a.currencyMismatch = true
				continue
			}
			a.add(&a.billed, b.AmountCents)
			a.add(&a.billedUnits, b.Quantity)
		}
		if a.currencyMismatch {
			summary.SkippedCurrencies++
			continue
		}
		if a.overflow {
			summary.SkippedOverflow++
			continue
		}
		summary.EligiblePairs++
		chargeable := a.usage - c.IncludedUnits
		if chargeable < 0 {
			chargeable = 0
		}
		expected, valid := safeProduct(chargeable, c.UnitPriceCents)
		if !valid {
			summary.SkippedOverflow++
			continue
		}
		if expected < c.MonthlyMinimumCents {
			expected = c.MonthlyMinimumCents
		}
		expected -= a.credits
		if expected < 0 {
			expected = 0
		}
		gap := expected - a.billed
		if gap <= 0 {
			continue
		}
		kind := "price_mismatch"
		switch {
		case len(a.invoiceIDs) == 0:
			kind = "missing_invoice"
		case a.billedUnits < chargeable:
			kind = "unbilled_usage"
		case chargeable*c.UnitPriceCents < c.MonthlyMinimumCents:
			kind = "minimum_shortfall"
		}
		f := Finding{ID: findingID(k), CustomerID: k.customer, CustomerName: c.CustomerName, Period: k.period, Metric: k.metric, Kind: kind, Currency: strings.ToUpper(c.Currency), PotentialCents: gap, ExpectedCents: expected, BilledCents: a.billed, CreditedCents: a.credits, UsageUnits: a.usage, BilledUnits: a.billedUnits, IncludedUnits: c.IncludedUnits, UnitPriceCents: c.UnitPriceCents, MonthlyMinimumCents: c.MonthlyMinimumCents, EvidenceUsageIDs: unique(a.uIDs), EvidenceInvoiceIDs: unique(a.invoiceIDs), Status: "new", FirstSeen: summary.At, LastSeen: summary.At}
		f.Explanation = fmt.Sprintf("%s: %d recorded %s units (%d included); expected %d %s cents after %d cents in explicit credits, versus %d cents on finalized invoices. Review contract terms, invoice period, discounts, and source completeness before attempting recovery.", c.CustomerName, a.usage, k.metric, c.IncludedUnits, expected, f.Currency, a.credits, a.billed)
		if old, ok := previous[f.ID]; ok {
			f.Status = old.Status
			f.Note = old.Note
			f.FirstSeen = old.FirstSeen
		}
		findings = append(findings, f)
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].PotentialCents > findings[j].PotentialCents })
	d.Findings = findings
	summary.Findings = len(findings)
	d.LastScan = summary
	return summary
}
func unique(in []string) []string {
	seen := map[string]bool{}
	a := []string{}
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			a = append(a, s)
		}
	}
	sort.Strings(a)
	return a
}
