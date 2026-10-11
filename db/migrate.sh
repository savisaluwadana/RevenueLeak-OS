#!/usr/bin/env bash
# Apply versioned PostgreSQL migrations to an explicitly configured database.
# Usage: DATABASE_URL='postgresql://...' bash db/migrate.sh
set -euo pipefail
if [[ -z "${DATABASE_URL:-}" ]]; then
  echo "DATABASE_URL is required; no migration was attempted" >&2
  exit 2
fi
if ! command -v psql >/dev/null 2>&1; then
  echo "psql (PostgreSQL client) is required" >&2
  exit 2
fi
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
checksum_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    echo "sha256sum or shasum is required" >&2
    return 2
  fi
}
# The migration ledger is intentionally owner-only and outside the app's RLS
# tables. Never grant the runtime role write access to this ledger.
psql "$DATABASE_URL" -X -v ON_ERROR_STOP=1 -q -c "
  CREATE SCHEMA IF NOT EXISTS revenueleak;
  CREATE TABLE IF NOT EXISTS revenueleak.schema_migrations (
    file_name text PRIMARY KEY,
    checksum char(64) NOT NULL,
    applied_at timestamptz NOT NULL DEFAULT now()
  );"
shopt -s nullglob
files=("$root"/db/migrations/[0-9][0-9][0-9][0-9]_*.sql)
if (( ${#files[@]} == 0 )); then
  echo "No migrations found" >&2
  exit 2
fi
for file in "${files[@]}"; do
  name="$(basename "$file")"
  if [[ ! "$name" =~ ^[0-9]{4}_[a-z0-9_]+\.sql$ ]]; then
    echo "Invalid migration filename: $name" >&2
    exit 2
  fi
  checksum="$(checksum_of "$file")"
  applied="$(psql "$DATABASE_URL" -X -v ON_ERROR_STOP=1 -qAt -c "SELECT trim(checksum) FROM revenueleak.schema_migrations WHERE file_name='$name'")"
  if [[ -n "$applied" ]]; then
    if [[ "$applied" != "$checksum" ]]; then
      echo "Migration drift: $name has changed after application" >&2
      exit 1
    fi
    echo "Already applied: $name"
    continue
  fi
  echo "Applying: $name"
  # -1 makes both the migration and its ledger entry one transaction.
  # A failed statement aborts the entire migration; previous data remains.
  psql "$DATABASE_URL" -X -v ON_ERROR_STOP=1 -1 -q \
    -f "$file" \
    -c "INSERT INTO revenueleak.schema_migrations(file_name,checksum) VALUES ('$name','$checksum');"
done
echo "Database migrations complete"
