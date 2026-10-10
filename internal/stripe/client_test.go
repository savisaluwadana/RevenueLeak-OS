package stripe

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchMappedMonthlyLines(t *testing.T) {
	start, _ := time.Parse("2006-01-02", "2026-08-01")
	end := start.AddDate(0, 1, 0)
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer rk_test" {
			t.Errorf("auth: %q", got)
		}
		if strings.HasPrefix(r.URL.Path, "/v1/invoices/in_1/lines") {
			fmt.Fprintf(w, `{"data":[{"id":"il_1","amount":2000,"quantity":200,"currency":"usd","period":{"start":%d,"end":%d},"pricing":{"price_details":{"price":"price_calls"}},"discount_amounts":[{"amount":500}]},{"id":"il_2","amount":1000,"quantity":10,"currency":"usd","period":{"start":%d,"end":%d},"price":{"id":"price_unknown"}}],"has_more":false}`, start.Unix(), end.Unix(), start.Unix(), end.Unix())
			return
		}
		if r.URL.Path == "/v1/invoices" {
			fmt.Fprint(w, `{"data":[{"id":"in_1","customer":"cus_acme","status":"paid","currency":"usd"}],"has_more":false}`)
			return
		}
		t.Errorf("unexpected path: %s", r.URL.Path)
		http.NotFound(w, r)
	}))
	defer testServer.Close()
	c := &Client{Key: "rk_test", BaseURL: testServer.URL, PriceMap: map[string]string{"price_calls": "api_calls"}, HTTP: testServer.Client()}
	got, err := c.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Invoices != 1 || got.Lines != 1 || got.SkippedUnmapped != 1 || len(got.Records) != 1 || got.Records[0].AmountCents != 1500 || got.Records[0].Period != "2026-08" {
		t.Fatalf("unexpected result: %+v", got)
	}
}
func TestStripeErrorNotExposingResponseBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		fmt.Fprint(w, "api key SECRET EXPOSED")
	}))
	defer srv.Close()
	c := &Client{Key: "rk_test", BaseURL: srv.URL, PriceMap: map[string]string{"p": "m"}, HTTP: srv.Client()}
	_, err := c.Fetch(context.Background())
	if err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("secret leaked or error missing: %v", err)
	}
}
