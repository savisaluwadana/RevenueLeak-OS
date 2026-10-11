//go:build postgres_integration

package pgstore

import (
	"errors"
	"os"
	"strings"
	"testing"

	"revenueleak-os/internal/core"
)

const alphaID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
const betaID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"

func openTestStore(t *testing.T, id string) *Store {
	t.Helper()
	dsn := os.Getenv("POSTGRES_TEST_URL")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_URL is not set")
	}
	s, err := Open(dsn, id)
	if err != nil {
		t.Fatalf("open database workspace: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestPostgreSQLTenantDocumentsAndRollback(t *testing.T) {
	alpha := openTestStore(t, alphaID)
	beta := openTestStore(t, betaID)
	a, err := alpha.Read()
	if err != nil {
		t.Fatal(err)
	}
	b, err := beta.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Contracts) != 0 || len(b.Contracts) != 0 {
		t.Fatal("test workspace was not initially empty")
	}

	if err := alpha.Update(func(d *core.Database) error {
		d.Contracts = append(d.Contracts, core.Contract{
			CustomerID: "shared-id", CustomerName: "Alpha Customer",
			Metric: "api_calls", Currency: "USD", UnitPriceCents: 10,
			EffectiveFrom: "2026-01",
		})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := beta.Update(func(d *core.Database) error {
		d.Contracts = append(d.Contracts, core.Contract{
			CustomerID: "shared-id", CustomerName: "Beta Customer",
			Metric: "api_calls", Currency: "USD", UnitPriceCents: 15,
			EffectiveFrom: "2026-01",
		})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	afterA, err := alpha.Read()
	if err != nil || len(afterA.Contracts) != 1 || afterA.Contracts[0].CustomerName != "Alpha Customer" {
		t.Fatalf("alpha read contains foreign or missing data: %+v %v", afterA, err)
	}
	afterB, err := beta.Read()
	if err != nil || len(afterB.Contracts) != 1 || afterB.Contracts[0].CustomerName != "Beta Customer" {
		t.Fatalf("beta read contains foreign or missing data: %+v %v", afterB, err)
	}
	abort := errors.New("reject mutation")
	err = alpha.Update(func(d *core.Database) error {
		d.Contracts[0].CustomerName = "CORRUPTED"
		return abort
	})
	if !errors.Is(err, abort) {
		t.Fatalf("callback failure was swallowed: %v", err)
	}
	snapshot, err := alpha.Read()
	if err != nil || snapshot.Contracts[0].CustomerName != "Alpha Customer" {
		t.Fatalf("failed transaction modified persisted finance records: %+v %v", snapshot, err)
	}
	reopened := openTestStore(t, alphaID)
	snapshot, err = reopened.Read()
	if err != nil || snapshot.Contracts[0].CustomerName != "Alpha Customer" {
		t.Fatalf("workspace data did not survive new database handle: %+v %v", snapshot, err)
	}
}

func TestPostgreSQLNonexistentOrganizationFailsClosed(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_URL")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_URL is not set")
	}
	_, err := Open(dsn, "cccccccc-cccc-4ccc-8ccc-cccccccccccc")
	if err == nil || !strings.Contains(err.Error(), "workspace has not been provisioned") {
		t.Fatalf("missing organization should be rejected, got %v", err)
	}
}

func TestPostgreSQLClosedConnectionNeverReturnsCachedData(t *testing.T) {
	s := openTestStore(t, alphaID)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if d, err := s.Read(); err == nil {
		t.Fatalf("closed database returned apparent financial data: %+v", d)
	}
}
