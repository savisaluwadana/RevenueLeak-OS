// Package pgstore persists the existing pilot workspace document in
// PostgreSQL, using transaction-local organization scoping and forced RLS.
// This is a private-workspace adapter, NOT multi-user tenant authorization.
package pgstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/lib/pq"
	"revenueleak-os/internal/core"
)

const maxWriteAttempts = 3

var uuidPattern = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)

type Store struct {
	db             *sql.DB
	organizationID string
}

var _ core.Repository = (*Store)(nil)

// Open requires a previously provisioned organization and an unprivileged DB
// login. A schema owner, superuser or BYPASSRLS role is explicitly refused.
func Open(dsn, organizationID string) (*Store, error) {
	if dsn == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	if !uuidPattern.MatchString(organizationID) {
		return nil, errors.New("WORKSPACE_ID must be a valid UUID")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL: %w", err)
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(30 * time.Minute)
	s := &Store{db: db, organizationID: organizationID}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}
	var superuser, bypassRLS, tableOwner bool
	err = db.QueryRowContext(ctx, `SELECT r.rolsuper, r.rolbypassrls,
		(SELECT c.relowner = r.oid
		 FROM pg_class c
		 WHERE c.oid = 'revenueleak.workspace_documents'::regclass)
		FROM pg_roles r WHERE r.rolname = current_user`).Scan(&superuser, &bypassRLS, &tableOwner)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("verify PostgreSQL runtime role: %w", err)
	}
	if superuser || bypassRLS || tableOwner {
		db.Close()
		return nil, errors.New("PostgreSQL runtime role must not own the schema, be superuser or bypass row-level security")
	}
	if _, err = s.Read(); err != nil {
		db.Close()
		return nil, fmt.Errorf("verify workspace access: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) begin(ctx context.Context, readOnly bool) (*sql.Tx, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable, ReadOnly: readOnly})
	if err != nil {
		return nil, err
	}
	var applied string
	if err = tx.QueryRowContext(ctx, "SELECT set_config('app.organization_id', $1, true)", s.organizationID).Scan(&applied); err != nil {
		tx.Rollback()
		return nil, err
	}
	if applied != s.organizationID {
		tx.Rollback()
		return nil, errors.New("PostgreSQL transaction organization context was not applied")
	}
	var id string
	err = tx.QueryRowContext(ctx, "SELECT id FROM revenueleak.organizations WHERE id = $1", s.organizationID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		tx.Rollback()
		return nil, errors.New("workspace has not been provisioned")
	}
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	return tx, nil
}

// Read retrieves an organization-scoped snapshot fresh from PostgreSQL.
// It never substitutes a cached dataset when the database fails.
func (s *Store) Read() (core.Database, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tx, err := s.begin(ctx, true)
	if err != nil {
		return core.Database{}, err
	}
	defer tx.Rollback()
	var encoded []byte
	err = tx.QueryRowContext(ctx,
		"SELECT state FROM revenueleak.workspace_documents WHERE organization_id = $1",
		s.organizationID).Scan(&encoded)
	var d core.Database
	if errors.Is(err, sql.ErrNoRows) {
		d = core.NewDatabase()
	} else if err != nil {
		return core.Database{}, err
	} else if err = json.Unmarshal(encoded, &d); err != nil {
		return core.Database{}, fmt.Errorf("decode workspace state: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return core.Database{}, err
	}
	return d, nil
}

// Update locks the organization's single workspace document and atomically
// commits the complete snapshot. Serialization failures retry a pure callback;
// external network/financial side effects must remain OUTSIDE the callback.
func (s *Store) Update(fn func(*core.Database) error) error {
	for attempt := 0; attempt < maxWriteAttempts; attempt++ {
		err := s.updateOnce(fn)
		if err == nil {
			return nil
		}
		var dbError *pq.Error
		if !errors.As(err, &dbError) || (string(dbError.Code) != "40001" && string(dbError.Code) != "40P01") {
			return err
		}
		if attempt+1 == maxWriteAttempts {
			return fmt.Errorf("PostgreSQL write conflict after retries: %w", err)
		}
		time.Sleep(time.Duration(attempt+1) * 25 * time.Millisecond)
	}
	return errors.New("PostgreSQL update attempts exhausted")
}

func (s *Store) updateOnce(fn func(*core.Database) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tx, err := s.begin(ctx, false)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// First write by a provisioned organization creates its private document.
	// ON CONFLICT is safe when two writers race to create the initial row.
	_, err = tx.ExecContext(ctx, `INSERT INTO revenueleak.workspace_documents (organization_id,state)
		VALUES ($1,'{}'::jsonb) ON CONFLICT (organization_id) DO NOTHING`, s.organizationID)
	if err != nil {
		return err
	}
	var encoded []byte
	err = tx.QueryRowContext(ctx, `SELECT state FROM revenueleak.workspace_documents
		WHERE organization_id=$1 FOR UPDATE`, s.organizationID).Scan(&encoded)
	if err != nil {
		return err
	}
	d := core.NewDatabase()
	if string(encoded) != "{}" {
		if err = json.Unmarshal(encoded, &d); err != nil {
			return fmt.Errorf("decode workspace before mutation: %w", err)
		}
	}
	if err = fn(&d); err != nil {
		return err
	}
	d.UpdatedAt = core.Stamp()
	payload, err := json.Marshal(d)
	if err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `UPDATE revenueleak.workspace_documents
		SET state=$2::jsonb, version=version+1, updated_at=now()
		WHERE organization_id=$1`, s.organizationID, payload)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return errors.New("workspace mutation affected no rows")
	}
	return tx.Commit()
}
