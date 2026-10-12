# Local development

Use Go 1.23 or newer. The backend adds a pinned PostgreSQL driver dependency (download with `go mod download`); the frontend uses browser JavaScript without runtime packages.

```bash
./start-demo.sh
```

Open http://127.0.0.1:8080. This seeds synthetic records and writes the ignored `data/revenueleak.json` file. Use another port by running the server directly:

```bash
DEMO_MODE=true ADDR=127.0.0.1:8081 go run ./cmd/server
```

For a private workspace, copy `.env.example` to `.env`, replace the password and session secret, and run `docker compose up --build`. Compose loads that file; `go run` and the demo launcher do not implicitly load `.env`.

```bash
go test -race ./...
go vet ./...
go build -o revenueleak ./cmd/server
node --check web/app.js
gofmt -w cmd internal
```

For the browser smoke suite (optional locally, required in CI), use Node 24, then:

```bash
npm ci
npx playwright install chromium
go build -o /tmp/revenueleak-next ./cmd/server
npm run test:browser
```

Set `REVENUELEAK_TEST_BINARY` if the compiled server is elsewhere. The suite creates an isolated temporary datastore and synthetic credentials, verifies desktop/mobile navigation, evidence downloads, review invalidation and restart persistence, then removes its test data. Node dependencies are test tooling only; the app still runs with Go alone. CI also builds the Docker image, validates Compose and invokes the executable demo launcher. After changing financial rules, add explicit expected-output fixtures for the changed contract behavior. Test unchanged cases, changed evidence, invalid records, persistence rollback and unauthorized exports.

For manual QA: run a scan, inspect Saved runs, download a JSON bundle, change a usage CSV, then inspect both the new result and the old run. The old export must remain identical; reviewed cases whose evidence changed must reopen. Restart the server and confirm history persists. Use synthetic data only for repository test fixtures.

Make development changes on a feature branch and open a PR. Keep roadmap and supported contract limitations accurate; a passing CI run is not production or finance sign-off.

## Optional automatic audits

`AUTO_SCAN_INTERVAL=24h` enables single-process periodic reconciliation, with a minimum interval of one hour. The initial due-check is performed when the server starts. Duplicate same-day scans with identical source digests and rule versions do not create another saved run. Re-checks are triggered when source data changes or a new UTC date begins. This does not fetch fresh provider data or guarantee billing completeness. Avoid running more than one instance with the JSON datastore; the production design requires durable distributed jobs, PostgreSQL and tenant isolation. Stop the process using SIGTERM/SIGINT to exercise graceful shutdown.

## PostgreSQL schema checks (not the running data store)

For a **disposable** PostgreSQL 15+ database, set `DATABASE_URL` to a migration-owner connection and run `bash db/migrate.sh`. The script requires `psql` and a SHA-256 command. CI runs this migration twice, rejects checksum drift and executes the two-organization RLS test fixture in `db/tests/tenant_isolation.sql` using PostgreSQL 16. The default local demo remains independent of Postgres and keeps storing data in the single-workspace JSON file; `DATABASE_URL` and `WORKSPACE_ID` enable opt-in private PostgreSQL storage. See [PostgreSQL foundation](POSTGRESQL-FOUNDATION.md) for limitations and the required app-side authorization work.

## Persistence error contract

The HTTP server and scheduled scanner now depend on the `core.Repository` interface (`Read` returns both a snapshot and an error; `Update` is atomic). This is the first boundary required for PostgreSQL integration, not a PostgreSQL connection. If a future adapter cannot read its database, the dashboard, investigations and evidence exports must return an explicit 500 error rather than silently inventing an empty portfolio or a missing-history 404. Failure-injection tests cover the behavior; the existing local JSON store remains the runtime implementation.

## PostgreSQL private workspace integration tests

Run `go test -race -tags=postgres_integration ./internal/pgstore` against a disposable PostgreSQL 16 database after migrations and fixture setup. Set `POSTGRES_TEST_URL` to a non-superuser database login with SELECT on organizations and SELECT/INSERT/UPDATE on workspace_documents, plus schema/function usage. The GitHub workflow provisions two test organizations and exercises database-backed storage separation, rollback and persistence automatically. These tests do not cover multi-user HTTP authorization or production backup/restore.

## One-time pilot data migration

Use `go run ./cmd/migrate-pilot -source path/to/revenueleak.json` to inspect a paused pilot's JSON snapshot in **dry-run** mode. Explicit `-apply` additionally requires `DATABASE_URL` and `WORKSPACE_ID` for a previously provisioned and uninitialized private PostgreSQL organization. Take backups and stop all writers before applying. The CLI rejects overwrites and verifies the imported snapshot by reading it back and comparing SHA-256. For operational setup and recovery precautions, see [PostgreSQL foundation](POSTGRESQL-FOUNDATION.md#one-time-import-from-an-existing-json-pilot). PostgreSQL integration tests validate the import under the dedicated CI database role; no normalized relational conversion or multi-tenant session authorization is involved.

## Private PostgreSQL backup-and-restore integration

`go run ./cmd/backup-workspace -output /restricted/directory/unique-backup.json` exports the provisioned private `DATABASE_URL` + `WORKSPACE_ID` state into a never-overwritten mode-0600 JSON backup with a verified SHA-256 digest. Use the one-time `cmd/migrate-pilot` import against a **fresh organization**, not the source, to test recovery. CI provisions separate PostgreSQL fixture organizations and runs an end-to-end `postgres_integration` test of backup + re-import without overwriting live data. The backup is plaintext JSON and must be encrypted by the operator; it is not managed cloud backup/PITR.

## Finance recovery assertion feature

The finding drawer includes operator-entered confirmation and recorded collection assertions, distinct from calculated potential revenue leakage. The API routes are GET `/api/recoveries` and POST `/api/findings/{id}/recovery`. See `internal/core/recovery.go`, `internal/core/recovery_test.go`, `internal/server/recovery.go` and the authenticated/CSRF tests. The app rejects duplicate idempotency requests with altered content, outdated evidence fingerprints, unapproved findings, over-confirmation and over-collection. Reconciliation does **not** treat these records as billable invoice sources. The underlying JSON/JSONB remains mutable by storage administrators; no bank verification or individual user attribution exists.

The full [documentation index](README.md) links product handbook, architecture diagrams, data design, workflows, agent/connector proposal, security runbooks, testing and implementation plan. Updating any feature requires code/tests first, then reflecting current-versus-planned status in the reference docs.
