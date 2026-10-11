package core

import (
	"errors"
	"strings"
	"testing"
)

type failingReadRepo struct {
	updates int
}

func (r *failingReadRepo) Read() (Database, error) {
	return Database{}, errors.New("database connection failed")
}
func (r *failingReadRepo) Update(fn func(*Database) error) error {
	r.updates++
	return nil
}

func TestScheduledScanFailsClosedOnStorageReadFailure(t *testing.T) {
	r := &failingReadRepo{}
	ran, err := ScheduledScan(r, timeAt("2026-10-08"), 7)
	if ran || err == nil || !strings.Contains(err.Error(), "database connection failed") {
		t.Fatalf("scheduler concealed storage failure: ran=%v err=%v", ran, err)
	}
	if r.updates != 0 {
		t.Fatal("scheduler must never write after a failed read")
	}
}

func TestJSONStoreImplementsRepositoryRead(t *testing.T) {
	s, err := Open(t.TempDir() + "/store.json")
	if err != nil {
		t.Fatal(err)
	}
	var r Repository = s
	d, err := r.Read()
	if err != nil || len(d.Findings) != 0 {
		t.Fatalf("unexpected JSON store read: %+v %v", d, err)
	}
}
