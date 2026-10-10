package core

import "time"

type Contract struct {
	CustomerID          string `json:"customer_id"`
	CustomerName        string `json:"customer_name"`
	Metric              string `json:"metric"`
	Currency            string `json:"currency"`
	UnitPriceCents      int64  `json:"unit_price_cents"`
	IncludedUnits       int64  `json:"included_units"`
	MonthlyMinimumCents int64  `json:"monthly_minimum_cents"`
	EffectiveFrom       string `json:"effective_from"`
	EffectiveTo         string `json:"effective_to,omitempty"`
}
type Usage struct {
	ID         string `json:"usage_id"`
	CustomerID string `json:"customer_id"`
	Period     string `json:"period"`
	Metric     string `json:"metric"`
	Quantity   int64  `json:"quantity"`
}
type Billing struct {
	ID          string `json:"line_id"`
	InvoiceID   string `json:"invoice_id"`
	CustomerID  string `json:"customer_id"`
	Period      string `json:"period"`
	Metric      string `json:"metric"`
	Quantity    int64  `json:"quantity"`
	AmountCents int64  `json:"amount_cents"`
	Currency    string `json:"currency"`
	Status      string `json:"status"`
	Source      string `json:"source"`
}
type Credit struct {
	ID          string `json:"credit_id"`
	CustomerID  string `json:"customer_id"`
	Period      string `json:"period"`
	Metric      string `json:"metric"`
	AmountCents int64  `json:"amount_cents"`
	Reason      string `json:"reason"`
}
type Finding struct {
	ID                  string   `json:"id"`
	CustomerID          string   `json:"customer_id"`
	CustomerName        string   `json:"customer_name"`
	Period              string   `json:"period"`
	Metric              string   `json:"metric"`
	Kind                string   `json:"kind"`
	Currency            string   `json:"currency"`
	PotentialCents      int64    `json:"potential_cents"`
	ExpectedCents       int64    `json:"expected_cents"`
	BilledCents         int64    `json:"billed_cents"`
	CreditedCents       int64    `json:"credited_cents"`
	UsageUnits          int64    `json:"usage_units"`
	BilledUnits         int64    `json:"billed_units"`
	IncludedUnits       int64    `json:"included_units"`
	UnitPriceCents      int64    `json:"unit_price_cents"`
	MonthlyMinimumCents int64    `json:"monthly_minimum_cents"`
	EvidenceUsageIDs    []string `json:"evidence_usage_ids"`
	EvidenceInvoiceIDs  []string `json:"evidence_invoice_ids"`
	Explanation         string   `json:"explanation"`
	Status              string   `json:"status"`
	Note                string   `json:"note"`
	FirstSeen           string   `json:"first_seen"`
	LastSeen            string   `json:"last_seen"`
}
type AuditEvent struct {
	ID     string `json:"id"`
	At     string `json:"at"`
	Action string `json:"action"`
	Detail string `json:"detail"`
}
type ScanSummary struct {
	At                 string `json:"at"`
	EligiblePairs      int    `json:"eligible_pairs"`
	SkippedOpenPeriods int    `json:"skipped_open_periods"`
	SkippedNoContract  int    `json:"skipped_no_contract"`
	SkippedCurrencies  int    `json:"skipped_currencies"`
	SkippedOverflow    int    `json:"skipped_overflow"`
	Findings           int    `json:"findings"`
}
type Database struct {
	Contracts []Contract   `json:"contracts"`
	Usage     []Usage      `json:"usage"`
	Billing   []Billing    `json:"billing"`
	Credits   []Credit     `json:"credits"`
	Findings  []Finding    `json:"findings"`
	Audit     []AuditEvent `json:"audit"`
	LastScan  ScanSummary  `json:"last_scan"`
	UpdatedAt string       `json:"updated_at"`
}

func NewDatabase() Database {
	return Database{Contracts: []Contract{}, Usage: []Usage{}, Billing: []Billing{}, Credits: []Credit{}, Findings: []Finding{}, Audit: []AuditEvent{}}
}
func Stamp() string { return time.Now().UTC().Format(time.RFC3339) }
