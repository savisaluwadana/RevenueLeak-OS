package stripe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"revenueleak-os/internal/core"
)

type Client struct {
	Key      string
	BaseURL  string
	PriceMap map[string]string
	HTTP     *http.Client
}
type stripeList[T any] struct {
	Data    []T  `json:"data"`
	HasMore bool `json:"has_more"`
}
type invoice struct {
	ID       string          `json:"id"`
	Customer json.RawMessage `json:"customer"`
	Currency string          `json:"currency"`
	Status   string          `json:"status"`
}
type invoiceLine struct {
	ID       string `json:"id"`
	Amount   int64  `json:"amount"`
	Quantity *int64 `json:"quantity"`
	Currency string `json:"currency"`
	Period   struct {
		Start int64 `json:"start"`
		End   int64 `json:"end"`
	} `json:"period"`
	Price *struct {
		ID string `json:"id"`
	} `json:"price"`
	Pricing *struct {
		PriceDetails *struct {
			Price string `json:"price"`
		} `json:"price_details"`
	} `json:"pricing"`
	DiscountAmounts []struct {
		Amount int64 `json:"amount"`
	} `json:"discount_amounts"`
}
type Result struct {
	Invoices          int            `json:"invoices"`
	Lines             int            `json:"lines"`
	SkippedUnmapped   int            `json:"skipped_unmapped"`
	SkippedNonmonthly int            `json:"skipped_nonmonthly"`
	SkippedOther      int            `json:"skipped_other"`
	Records           []core.Billing `json:"-"`
}

func NewFromEnv() (*Client, error) {
	key := os.Getenv("STRIPE_RESTRICTED_KEY")
	if key == "" {
		return nil, errors.New("STRIPE_RESTRICTED_KEY is not configured on the server")
	}
	pm := map[string]string{}
	raw := os.Getenv("STRIPE_PRICE_MAP")
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &pm); err != nil {
			return nil, fmt.Errorf("invalid STRIPE_PRICE_MAP: %w", err)
		}
	}
	if len(pm) == 0 {
		return nil, errors.New("STRIPE_PRICE_MAP must explicitly map Stripe price IDs to usage metrics")
	}
	return &Client{Key: key, BaseURL: "https://api.stripe.com", PriceMap: pm, HTTP: &http.Client{Timeout: 20 * time.Second}}, nil
}
func (c *Client) get(ctx context.Context, path string, v any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(c.BaseURL, "/")+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Key)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
		return fmt.Errorf("stripe returned HTTP %d", res.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(v)
}
func customerID(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var obj struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &obj)
	return obj.ID
}
func (c *Client) Fetch(ctx context.Context) (Result, error) {
	out := Result{Records: []core.Billing{}}
	cursor := ""
	for page := 0; page < 20; page++ {
		path := "/v1/invoices?limit=100"
		if cursor != "" {
			path += "&starting_after=" + url.QueryEscape(cursor)
		}
		var invs stripeList[invoice]
		if err := c.get(ctx, path, &invs); err != nil {
			return Result{}, err
		}
		for _, inv := range invs.Data {
			if inv.Status != "paid" && inv.Status != "open" {
				continue
			}
			out.Invoices++
			if customerID(inv.Customer) == "" {
				out.SkippedOther++
				continue
			}
			lineCursor := ""
			for lp := 0; lp < 20; lp++ {
				lpath := "/v1/invoices/" + url.PathEscape(inv.ID) + "/lines?limit=100"
				if lineCursor != "" {
					lpath += "&starting_after=" + url.QueryEscape(lineCursor)
				}
				var lines stripeList[invoiceLine]
				if err := c.get(ctx, lpath, &lines); err != nil {
					return Result{}, err
				}
				for _, line := range lines.Data {
					pid := ""
					if line.Price != nil {
						pid = line.Price.ID
					}
					if pid == "" && line.Pricing != nil && line.Pricing.PriceDetails != nil {
						pid = line.Pricing.PriceDetails.Price
					}
					metric, ok := c.PriceMap[pid]
					if !ok {
						out.SkippedUnmapped++
						continue
					}
					if line.Period.Start <= 0 || line.Period.End <= line.Period.Start {
						out.SkippedOther++
						continue
					}
					start := time.Unix(line.Period.Start, 0).UTC()
					end := time.Unix(line.Period.End, 0).UTC()
					// Only compare full UTC calendar months. Skip prorations and nonmonthly billing.
					if start.Day() != 1 || start.Hour() != 0 || start.Minute() != 0 || start.Second() != 0 || !end.Equal(start.AddDate(0, 1, 0)) {
						out.SkippedNonmonthly++
						continue
					}
					if line.Quantity == nil {
						out.SkippedOther++
						continue
					}
					qty := *line.Quantity
					if qty < 0 || line.Amount < 0 {
						out.SkippedOther++
						continue
					}
					net := line.Amount
					for _, d := range line.DiscountAmounts {
						net -= d.Amount
					}
					if net < 0 {
						net = 0
					}
					cur := strings.ToUpper(line.Currency)
					if cur == "" {
						cur = strings.ToUpper(inv.Currency)
					}
					if cur != "USD" && cur != "EUR" && cur != "GBP" {
						out.SkippedOther++
						continue
					}
					out.Records = append(out.Records, core.Billing{ID: "stripe:" + line.ID, InvoiceID: inv.ID, CustomerID: customerID(inv.Customer), Period: start.Format("2006-01"), Metric: metric, Quantity: qty, AmountCents: net, Currency: cur, Status: inv.Status, Source: "stripe"})
					out.Lines++
				}
				if !lines.HasMore {
					break
				}
				if len(lines.Data) == 0 {
					return Result{}, errors.New("stripe returned empty paginated invoice lines")
				}
				lineCursor = lines.Data[len(lines.Data)-1].ID
				if lp == 19 {
					return Result{}, errors.New("invoice line pagination limit exceeded")
				}
			}
		}
		if !invs.HasMore {
			break
		}
		if len(invs.Data) == 0 {
			return Result{}, errors.New("stripe returned empty paginated invoices")
		}
		cursor = invs.Data[len(invs.Data)-1].ID
		if page == 19 {
			return Result{}, errors.New("invoice pagination limit exceeded; narrow account before sync")
		}
	}
	return out, nil
}
