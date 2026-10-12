// Command backup-workspace exports one private PostgreSQL workspace as a
// restricted JSON file suitable for guarded re-import into a fresh workspace.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"revenueleak-os/internal/core"
	"revenueleak-os/internal/pgstore"
)

// writeBackup creates the final file using an atomic, no-clobber hard link.
// A temporary 0600 file is fsynced and read back before publication.
func writeBackup(destination string, snapshot core.Database) (string, error) {
	if destination == "" {
		return "", errors.New("-output is required")
	}
	payload, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode workspace: %w", err)
	}
	expected, err := fingerprint(snapshot)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(destination)
	f, err := os.CreateTemp(dir, ".revenueleak-backup-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return "", err
	}
	if _, err = f.Write(payload); err != nil {
		f.Close()
		return "", err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	verify, err := os.ReadFile(f.Name())
	if err != nil {
		return "", err
	}
	var reread core.Database
	if err = json.Unmarshal(verify, &reread); err != nil {
		return "", fmt.Errorf("backup read-back failed: %w", err)
	}
	actual, err := fingerprint(reread)
	if err != nil {
		return "", err
	}
	if actual != expected {
		return "", errors.New("backup read-back fingerprint mismatch; output not published")
	}
	// Link fails if destination exists, including if it is a symlink.
	// This prevents silently overwriting an older financial backup.
	if err = os.Link(f.Name(), destination); err != nil {
		return "", fmt.Errorf("publish backup without overwrite: %w", err)
	}
	// Best-effort directory durability; report a failure rather than claiming
	// the backup was durably published when a directory fsync fails.
	directory, err := os.Open(dir)
	if err != nil {
		return "", fmt.Errorf("backup file exists but directory sync is unavailable: %w", err)
	}
	defer directory.Close()
	if err = directory.Sync(); err != nil {
		return "", fmt.Errorf("backup file exists but directory sync failed: %w", err)
	}
	return expected, nil
}

func fingerprint(d core.Database) (string, error) {
	encoded, err := json.Marshal(d)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func main() {
	output := flag.String("output", "", "destination JSON backup file; must not exist")
	flag.Parse()
	if flag.NArg() != 0 || *output == "" {
		fmt.Fprintln(os.Stderr, "usage: backup-workspace -output /secure/backups/revenueleak-YYYYMMDD.json")
		os.Exit(2)
	}
	if os.Getenv("DATABASE_URL") == "" || os.Getenv("WORKSPACE_ID") == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL and WORKSPACE_ID are required")
		os.Exit(2)
	}
	store, err := pgstore.Open(os.Getenv("DATABASE_URL"), os.Getenv("WORKSPACE_ID"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "open private workspace: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()
	snapshot, err := store.Read()
	if err != nil {
		fmt.Fprintf(os.Stderr, "read workspace: %v\n", err)
		os.Exit(1)
	}
	checksum, err := writeBackup(*output, snapshot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "backup failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Backup written with mode 0600: %s\nSHA-256 (canonical JSON): %s\nRecords: %d contracts, %d usage, %d invoices, %d findings, %d saved runs\n",
		*output, checksum, len(snapshot.Contracts), len(snapshot.Usage), len(snapshot.Billing), len(snapshot.Findings), len(snapshot.ScanRuns))
	fmt.Println("Contains sensitive financial records. Encrypt and store off-site with restricted access; verify restoration into a fresh workspace before trusting this backup.")
}
