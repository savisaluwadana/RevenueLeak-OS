// Command migrate-pilot copies one paused single-workspace JSON pilot into a
// provisioned, empty PostgreSQL private workspace without deleting the source.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"revenueleak-os/internal/core"
	"revenueleak-os/internal/pgstore"
)

const maxSnapshotBytes = 64 << 20

func loadSnapshot(path string) (core.Database, error) {
	if path == "" {
		return core.Database{}, errors.New("-source path is required")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return core.Database{}, err
	}
	if !info.Mode().IsRegular() {
		return core.Database{}, errors.New("source must be a regular, non-symlink file")
	}
	if info.Size() == 0 || info.Size() > maxSnapshotBytes {
		return core.Database{}, fmt.Errorf("source file must be between 1 and %d bytes", maxSnapshotBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return core.Database{}, err
	}
	if len(bytes.TrimSpace(data)) == 0 || bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return core.Database{}, errors.New("source is not a workspace object")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var snapshot core.Database
	if err := decoder.Decode(&snapshot); err != nil {
		return core.Database{}, fmt.Errorf("decode source: %w", err)
	}
	var unexpected any
	if err := decoder.Decode(&unexpected); !errors.Is(err, io.EOF) {
		return core.Database{}, errors.New("source must contain exactly one JSON object")
	}
	if snapshot.ScanSequence < 0 || len(snapshot.ScanRuns) > core.MaxSavedScans {
		return core.Database{}, errors.New("invalid saved-run sequence or retention length")
	}
	seen := make(map[string]bool, len(snapshot.ScanRuns))
	for _, run := range snapshot.ScanRuns {
		if run.ID == "" || seen[run.ID] {
			return core.Database{}, errors.New("saved scan identifiers must be present and unique")
		}
		seen[run.ID] = true
	}
	return snapshot, nil
}

// snapshotDigest reports the same SHA-256 for equivalent JSON data, regardless
// of formatting or JSONB key ordering in PostgreSQL.
func snapshotDigest(d core.Database) (string, error) {
	encoded, err := json.Marshal(d)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func migrate(snapshot core.Database, target *pgstore.Store) error {
	beforeDigest, err := snapshotDigest(snapshot)
	if err != nil {
		return err
	}
	if err := target.ImportEmptyWorkspace(snapshot); err != nil {
		return err
	}
	got, err := target.Read()
	if err != nil {
		return fmt.Errorf("import committed, but independent read-back FAILED; investigate before using the workspace: %w", err)
	}
	afterDigest, err := snapshotDigest(got)
	if err != nil {
		return err
	}
	if beforeDigest != afterDigest {
		return fmt.Errorf("import committed but read-back hash differs; DO NOT USE target until investigated (source=%s, target=%s)", beforeDigest, afterDigest)
	}
	return nil
}

func main() {
	source := flag.String("source", "", "path to stopped pilot's JSON snapshot")
	apply := flag.Bool("apply", false, "perform one-time import into an empty PostgreSQL workspace; default is dry-run")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		os.Exit(2)
	}
	snapshot, err := loadSnapshot(*source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid source: %v\n", err)
		os.Exit(1)
	}
	digest, err := snapshotDigest(snapshot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "snapshot digest failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Source SHA-256: %s\nContracts: %d, usage: %d, billing: %d, credits: %d, mappings: %d\nFindings: %d, coverage gaps: %d, historical runs: %d, scan sequence: %d\n",
		digest, len(snapshot.Contracts), len(snapshot.Usage), len(snapshot.Billing), len(snapshot.Credits),
		len(snapshot.Mappings), len(snapshot.Findings), len(snapshot.Coverage), len(snapshot.ScanRuns), snapshot.ScanSequence)
	if !*apply {
		fmt.Println("DRY RUN: no database connection or changes. Use -apply only after stopping the pilot and backing up the source and destination.")
		return
	}
	dsn := os.Getenv("DATABASE_URL")
	orgID := os.Getenv("WORKSPACE_ID")
	if dsn == "" || orgID == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL and WORKSPACE_ID are required with -apply")
		os.Exit(2)
	}
	target, err := pgstore.Open(dsn, orgID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to open destination: %v\n", err)
		os.Exit(1)
	}
	defer target.Close()
	if err = migrate(snapshot, target); err != nil {
		fmt.Fprintf(os.Stderr, "import failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Verified PostgreSQL migration into provisioned workspace %s; source file is unchanged.\n", orgID)
}
