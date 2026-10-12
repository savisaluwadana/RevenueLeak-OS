package pgstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	"revenueleak-os/internal/core"
)

var ErrWorkspaceNotEmpty = errors.New("target workspace already contains state; migration refuses to overwrite it")

// ImportEmptyWorkspace copies one validated local pilot snapshot into a
// provisioned PostgreSQL workspace, preserving all findings, reviews and
// saved run evidence. This operation must never run on a live target server.
//
// It requires an uninitialized workspace document and writes all state in
// one serializable transaction. Even a cleared JSONB workspace with version
// above zero cannot be overwritten by this bootstrap import.
func (s *Store) ImportEmptyWorkspace(snapshot core.Database) error {
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("encode migration input: %w", err)
	}
	for attempt := 0; attempt < maxWriteAttempts; attempt++ {
		err = s.importEmptyOnce(payload)
		if err == nil {
			return nil
		}
		var dbErr *pq.Error
		if !errors.As(err, &dbErr) || (string(dbErr.Code) != "40001" && string(dbErr.Code) != "40P01") {
			return err
		}
		if attempt+1 == maxWriteAttempts {
			return fmt.Errorf("concurrent workspace import conflict: %w", err)
		}
		time.Sleep(time.Duration(attempt+1) * 25 * time.Millisecond)
	}
	return errors.New("workspace import retry limit reached")
}

func (s *Store) importEmptyOnce(payload []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tx, err := s.begin(ctx, false)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO revenueleak.workspace_documents (organization_id, state)
		VALUES ($1, '{}'::jsonb) ON CONFLICT (organization_id) DO NOTHING`, s.organizationID)
	if err != nil {
		return err
	}
	var version int64
	var existing []byte
	err = tx.QueryRowContext(ctx,
		`SELECT version, state FROM revenueleak.workspace_documents WHERE organization_id=$1 FOR UPDATE`,
		s.organizationID).Scan(&version, &existing)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("target workspace is not accessible")
	}
	if err != nil {
		return err
	}
	if version != 0 || string(existing) != "{}" {
		return ErrWorkspaceNotEmpty
	}
	result, err := tx.ExecContext(ctx, `UPDATE revenueleak.workspace_documents
		SET state=$2::jsonb, version=version+1, updated_at=now()
		WHERE organization_id=$1 AND version=0`, s.organizationID, payload)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrWorkspaceNotEmpty
	}
	return tx.Commit()
}
