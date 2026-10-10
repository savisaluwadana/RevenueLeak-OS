package core

import (
	"fmt"
	"time"
)

func SeedDemo(d *Database, now time.Time) {
	month := now.UTC().AddDate(0, -2, 0).Format("2006-01")
	profiles := []struct {
		id, name, metric                            string
		price, included, min, usage, billed, charge int64
	}{
		{"cus_atlas", "Atlas Cloud", "api_calls", 9, 10000, 0, 105000, 80000, 630000},
		{"cus_meridian", "Meridian AI", "tokens_1k", 12, 0, 0, 85000, 85000, 690000},
		{"cus_nova", "Nova Systems", "events", 4, 5000, 0, 62000, 42000, 148000},
		{"cus_solstice", "Solstice Dev", "seats", 2300, 0, 200000, 100, 100, 120000},
		{"cus_pulse", "Pulse Commerce", "api_calls", 5, 2000, 0, 40000, 0, 0},
		{"cus_river", "River Analytics", "tokens_1k", 18, 0, 0, 25000, 25000, 450000},
		{"cus_zenith", "Zenith Studio", "api_calls", 10, 0, 0, 22000, 22000, 220000},
	}
	for i, p := range profiles {
		d.Contracts = append(d.Contracts, Contract{CustomerID: p.id, CustomerName: p.name, Metric: p.metric, Currency: "USD", UnitPriceCents: p.price, IncludedUnits: p.included, MonthlyMinimumCents: p.min, EffectiveFrom: now.AddDate(-1, 0, 0).Format("2006-01")})
		d.Usage = append(d.Usage, Usage{ID: fmt.Sprintf("demo_usage_%d", i), CustomerID: p.id, Period: month, Metric: p.metric, Quantity: p.usage})
		if p.billed > 0 {
			d.Billing = append(d.Billing, Billing{ID: fmt.Sprintf("demo_line_%d", i), InvoiceID: fmt.Sprintf("in_demo_%d", i), CustomerID: p.id, Period: month, Metric: p.metric, Quantity: p.billed, AmountCents: p.charge, Currency: "USD", Status: "paid", Source: "demo"})
		}
	}
	// Additional synthetic history creates an informative trend without inventing real recoveries.
	for offset := 6; offset >= 3; offset-- {
		m := now.UTC().AddDate(0, -offset, 0).Format("2006-01")
		for i, p := range profiles[:4] {
			quantity := p.usage * int64(11-offset) / 10
			billedUnits := quantity - int64((8-offset)*(i+1)*1000)
			if billedUnits < 0 {
				billedUnits = 0
			}
			chargeable := billedUnits - p.included
			if chargeable < 0 {
				chargeable = 0
			}
			billedAmount := chargeable * p.price
			if i == 1 {
				billedAmount = billedAmount * 8 / 10
			}
			if i == 3 {
				billedAmount = billedAmount * 6 / 10
			}
			d.Usage = append(d.Usage, Usage{ID: fmt.Sprintf("demo_usage_%d_%d", offset, i), CustomerID: p.id, Period: m, Metric: p.metric, Quantity: quantity})
			d.Billing = append(d.Billing, Billing{ID: fmt.Sprintf("demo_line_%d_%d", offset, i), InvoiceID: fmt.Sprintf("in_demo_%d_%d", offset, i), CustomerID: p.id, Period: m, Metric: p.metric, Quantity: billedUnits, AmountCents: billedAmount, Currency: "USD", Status: "paid", Source: "demo"})
		}
	}
	Log(d, "demo.seed", "Synthetic demo dataset inserted; no real customer data")
	Scan(d, now, 7)
}
