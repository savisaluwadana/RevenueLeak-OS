package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Store struct {
	mu   sync.Mutex
	path string
	data Database
}

func Open(path string) (*Store, error) {
	s := &Store{path: path, data: NewDatabase()}
	b, err := os.ReadFile(path)
	if err == nil {
		if err = json.Unmarshal(b, &s.data); err != nil {
			return nil, fmt.Errorf("corrupt data store: %w", err)
		}
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if s.data.Contracts == nil {
		s.data.Contracts = []Contract{}
	}
	if s.data.Usage == nil {
		s.data.Usage = []Usage{}
	}
	if s.data.Billing == nil {
		s.data.Billing = []Billing{}
	}
	if s.data.Credits == nil {
		s.data.Credits = []Credit{}
	}
	if s.data.Findings == nil {
		s.data.Findings = []Finding{}
	}
	if s.data.Audit == nil {
		s.data.Audit = []AuditEvent{}
	}
	return s, nil
}
func (s *Store) Snapshot() Database {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.Marshal(s.data)
	var cp Database
	_ = json.Unmarshal(b, &cp)
	return cp
}
func (s *Store) Update(fn func(*Database) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.Marshal(s.data)
	var cp Database
	if err := json.Unmarshal(b, &cp); err != nil {
		return err
	}
	if err := fn(&cp); err != nil {
		return err
	}
	cp.UpdatedAt = Stamp()
	out, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(s.path), ".revenueleak-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err = file.Chmod(0600); err != nil {
		file.Close()
		return err
	}
	if _, err = file.Write(out); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = os.Rename(file.Name(), s.path); err != nil {
		return err
	}
	s.data = cp
	return nil
}
func Log(d *Database, action, detail string) {
	d.Audit = append([]AuditEvent{{ID: fmt.Sprintf("evt_%d_%d", len(d.Audit)+1, len(detail)), At: Stamp(), Action: action, Detail: detail}}, d.Audit...)
	if len(d.Audit) > 500 {
		d.Audit = d.Audit[:500]
	}
}
