package core

import (
	"fmt"
	"sort"
	"time"
)

// CoverageLookbackMonths bounds discovery of missing sources so an open-ended
// contract does not create an unbounded number of historical warnings.
const CoverageLookbackMonths = 12

// CoverageGap is a data-quality warning, NOT a monetary leakage finding.
// Even a signed minimum cannot establish recoverability without confirming
// the contract remains active, invoices, credits and source completeness.
type CoverageGap struct {
	ID                  string `json:"id"`
	CustomerID          string `json:"customer_id"`
	CustomerName        string `json:"customer_name"`
	Period              string `json:"period"`
	Metric              string `json:"metric"`
	Currency            string `json:"currency"`
	MonthlyMinimumCents int64  `json:"monthly_minimum_cents"`
	Reason              string `json:"reason"`
	Explanation         string `json:"explanation"`
}

// detectCoverageGaps checks completed periods that are older than the grace
// window. A contract-only period must not be reported as an actual debt or
// included in potential revenue totals: its data is not sufficient for math.
func detectCoverageGaps(d *Database, now time.Time, graceDays int) []CoverageGap {
	out := []CoverageGap{}
	cutoff := now.UTC().AddDate(0, 0, -graceDays)
	firstOfCutoffMonth := time.Date(cutoff.Year(), cutoff.Month(), 1, 0, 0, 0, 0, time.UTC)
	latestEligibleMonth := firstOfCutoffMonth.AddDate(0, -1, 0)

	// A finalized invoice OR a usage record provides evidence the pair is
	// present. Draft/void invoices are not finalized and do not close the gap.
	present := map[pair]bool{}
	aliases := mappingIndex(d.Mappings)
	for _, u := range d.Usage {
		present[pair{mappedCustomer(aliases, "usage_csv", u.CustomerID), u.Period, u.Metric}] = true
	}
	for _, b := range d.Billing {
		if b.Status == "paid" || b.Status == "open" {
			present[pair{mappedCustomer(aliases, billingSource(b.Source), b.CustomerID), b.Period, b.Metric}] = true
		}
	}

	uniqueContracts := map[pair]bool{}
	for _, c := range d.Contracts {
		if c.MonthlyMinimumCents > 0 {
			uniqueContracts[pair{c.CustomerID, "", c.Metric}] = true
		}
	}
	for contractKey := range uniqueContracts {
		for i := 0; i < CoverageLookbackMonths; i++ {
			period := latestEligibleMonth.AddDate(0, -i, 0).Format("2006-01")
			k := pair{contractKey.customer, period, contractKey.metric}
			if present[k] || !periodEligible(period, now, graceDays) {
				continue
			}
			c, ok := contractFor(d.Contracts, k)
			if !ok || c.MonthlyMinimumCents == 0 {
				continue
			}
			out = append(out, CoverageGap{
				ID:                  "coverage_" + findingID(k),
				CustomerID:          k.customer,
				CustomerName:        c.CustomerName,
				Period:              period,
				Metric:              k.metric,
				Currency:            c.Currency,
				MonthlyMinimumCents: c.MonthlyMinimumCents,
				Reason:              "missing_usage_and_finalized_billing",
				Explanation:         fmt.Sprintf("No independent usage or finalized invoice was provided for %s / %s / %s. The contract records a monthly minimum; confirm that the contract was active, all data was imported, and any credits or adjustments before estimating a shortfall.", c.CustomerName, period, k.metric),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Period != out[j].Period {
			return out[i].Period > out[j].Period
		}
		if out[i].CustomerID != out[j].CustomerID {
			return out[i].CustomerID < out[j].CustomerID
		}
		return out[i].Metric < out[j].Metric
	})
	return out
}
