package core

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

type ImportResult struct {
	Kind    string `json:"kind"`
	Added   int    `json:"added"`
	Updated int    `json:"updated"`
	Rows    int    `json:"rows"`
}

func ReadCSV(kind string, r io.Reader) (any, error) {
	parser := csv.NewReader(r)
	parser.FieldsPerRecord = -1
	parser.TrimLeadingSpace = true
	raw, err := parser.Read()
	if err != nil {
		return nil, fmt.Errorf("CSV header: %w", err)
	}
	heads := map[string]int{}
	for i, h := range raw {
		h = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\ufeff")))
		if _, ok := heads[h]; ok {
			return nil, fmt.Errorf("duplicate column %q", h)
		}
		heads[h] = i
	}
	required := map[string][]string{
		"contracts": {"customer_id", "customer_name", "metric", "currency", "unit_price_cents", "included_units", "monthly_minimum_cents", "effective_from"},
		"usage":     {"usage_id", "customer_id", "period", "metric", "quantity"},
		"billing":   {"line_id", "invoice_id", "customer_id", "period", "metric", "quantity", "amount_cents", "currency", "status"},
		"credits":   {"credit_id", "customer_id", "period", "metric", "amount_cents", "reason"},
	}
	req, ok := required[kind]
	if !ok {
		return nil, errors.New("invalid import kind")
	}
	for _, h := range req {
		if _, ok := heads[h]; !ok {
			return nil, fmt.Errorf("missing column %q", h)
		}
	}
	get := func(row []string, field string) string {
		index, ok := heads[field]
		if !ok || index >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[index])
	}
	num := func(row []string, col string) (int64, error) {
		n, e := strconv.ParseInt(get(row, col), 10, 64)
		if e != nil || n < 0 || n > 1_000_000_000_000 {
			return 0, fmt.Errorf("%s must be a nonnegative integer <= 1e12", col)
		}
		return n, nil
	}
	period := func(v string) bool { t, e := time.Parse("2006-01", v); return e == nil && t.Format("2006-01") == v }
	nonempty := func(row []string, fields ...string) error {
		for _, f := range fields {
			if get(row, f) == "" {
				return fmt.Errorf("%s cannot be empty", f)
			}
		}
		return nil
	}
	contracts := []Contract{}
	usage := []Usage{}
	billing := []Billing{}
	credits := []Credit{}
	for line := 2; ; line++ {
		row, e := parser.Read()
		if errors.Is(e, io.EOF) {
			break
		}
		if e != nil {
			return nil, fmt.Errorf("row %d: %w", line, e)
		}
		if line > 10001 {
			return nil, errors.New("import limited to 10000 rows")
		}
		allEmpty := true
		for _, v := range row {
			if strings.TrimSpace(v) != "" {
				allEmpty = false
				break
			}
		}
		if allEmpty {
			continue
		}
		fail := func(err error) (any, error) { return nil, fmt.Errorf("row %d: %w", line, err) }
		switch kind {
		case "contracts":
			if e = nonempty(row, "customer_id", "customer_name", "metric", "currency", "effective_from"); e != nil {
				return fail(e)
			}
			a, e := num(row, "unit_price_cents")
			if e != nil {
				return fail(e)
			}
			b, e := num(row, "included_units")
			if e != nil {
				return fail(e)
			}
			c, e := num(row, "monthly_minimum_cents")
			if e != nil {
				return fail(e)
			}
			if !period(get(row, "effective_from")) || (get(row, "effective_to") != "" && !period(get(row, "effective_to"))) {
				return fail(errors.New("effective dates must be YYYY-MM"))
			}
			if get(row, "effective_to") != "" && get(row, "effective_to") < get(row, "effective_from") {
				return fail(errors.New("effective_to before effective_from"))
			}
			cur := strings.ToUpper(get(row, "currency"))
			if cur != "USD" && cur != "EUR" && cur != "GBP" {
				return fail(errors.New("only USD, EUR, GBP supported (2-decimal currencies)"))
			}
			contracts = append(contracts, Contract{get(row, "customer_id"), get(row, "customer_name"), get(row, "metric"), cur, a, b, c, get(row, "effective_from"), get(row, "effective_to")})
		case "usage":
			if e = nonempty(row, "usage_id", "customer_id", "period", "metric"); e != nil {
				return fail(e)
			}
			if !period(get(row, "period")) {
				return fail(errors.New("period must be YYYY-MM"))
			}
			q, e := num(row, "quantity")
			if e != nil {
				return fail(e)
			}
			usage = append(usage, Usage{get(row, "usage_id"), get(row, "customer_id"), get(row, "period"), get(row, "metric"), q})
		case "billing":
			if e = nonempty(row, "line_id", "invoice_id", "customer_id", "period", "metric", "currency", "status"); e != nil {
				return fail(e)
			}
			if !period(get(row, "period")) {
				return fail(errors.New("period must be YYYY-MM"))
			}
			q, e := num(row, "quantity")
			if e != nil {
				return fail(e)
			}
			a, e := num(row, "amount_cents")
			if e != nil {
				return fail(e)
			}
			s := strings.ToLower(get(row, "status"))
			if s != "paid" && s != "open" && s != "draft" && s != "void" {
				return fail(errors.New("status must be paid/open/draft/void"))
			}
			cur := strings.ToUpper(get(row, "currency"))
			if cur != "USD" && cur != "EUR" && cur != "GBP" {
				return fail(errors.New("only USD, EUR, GBP supported"))
			}
			billing = append(billing, Billing{get(row, "line_id"), get(row, "invoice_id"), get(row, "customer_id"), get(row, "period"), get(row, "metric"), q, a, cur, s, "csv"})
		case "credits":
			if e = nonempty(row, "credit_id", "customer_id", "period", "metric", "reason"); e != nil {
				return fail(e)
			}
			if !period(get(row, "period")) {
				return fail(errors.New("period must be YYYY-MM"))
			}
			a, e := num(row, "amount_cents")
			if e != nil {
				return fail(e)
			}
			credits = append(credits, Credit{get(row, "credit_id"), get(row, "customer_id"), get(row, "period"), get(row, "metric"), a, get(row, "reason")})
		}
	}
	switch kind {
	case "contracts":
		return contracts, nil
	case "usage":
		return usage, nil
	case "billing":
		return billing, nil
	default:
		return credits, nil
	}
}
func Import(d *Database, kind string, value any) ImportResult {
	result := ImportResult{Kind: kind}
	switch kind {
	case "contracts":
		for _, v := range value.([]Contract) {
			found := false
			for i, old := range d.Contracts {
				if old.CustomerID == v.CustomerID && old.Metric == v.Metric && old.EffectiveFrom == v.EffectiveFrom {
					d.Contracts[i] = v
					found = true
					result.Updated++
					break
				}
			}
			if !found {
				d.Contracts = append(d.Contracts, v)
				result.Added++
			}
		}
	case "usage":
		for _, v := range value.([]Usage) {
			found := false
			for i, old := range d.Usage {
				if old.ID == v.ID {
					d.Usage[i] = v
					found = true
					result.Updated++
					break
				}
			}
			if !found {
				d.Usage = append(d.Usage, v)
				result.Added++
			}
		}
	case "billing":
		for _, v := range value.([]Billing) {
			found := false
			for i, old := range d.Billing {
				if old.ID == v.ID {
					d.Billing[i] = v
					found = true
					result.Updated++
					break
				}
			}
			if !found {
				d.Billing = append(d.Billing, v)
				result.Added++
			}
		}
	case "credits":
		for _, v := range value.([]Credit) {
			found := false
			for i, old := range d.Credits {
				if old.ID == v.ID {
					d.Credits[i] = v
					found = true
					result.Updated++
					break
				}
			}
			if !found {
				d.Credits = append(d.Credits, v)
				result.Added++
			}
		}
	}
	result.Rows = result.Added + result.Updated
	return result
}
