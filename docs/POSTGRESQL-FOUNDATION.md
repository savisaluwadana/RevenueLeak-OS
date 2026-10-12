# PostgreSQL foundation and private-workspace adapter

**Status:** PostgreSQL migrations and row-level-security (RLS) tests are implemented, and the Go server has an **opt-in private PostgreSQL workspace adapter**. The default demo remains local JSON. This is **not** multi-user or multi-tenant SaaS authorization: there is still one administrator credential per HTTP server and no OIDC/RBAC.

## Data architecture

Migration `0001_tenant_foundation.sql` creates tenant-scoped organizations, memberships, contracts, provider/customer mappings, usage, invoice lines, credits, reconciliation runs, findings, approvals, ingest jobs, and audit events. Every tenant-owned table is forced into RLS; financial cross-table foreign keys include organization identifiers.

Migration `0002_workspace_documents.sql` adds one transactionally versioned `workspace_documents` JSONB row per organization. The **current Go adapter uses this transitional row** to persist the existing `core.Database` snapshot, including its bounded run history. It does **not** yet write to the normalized relational tables in migration 0001. The normalized schema is for the next development phase.

`core.Repository` exposes `Read() (Database, error)` and `Update(func(*Database) error) error`. The JSON demo and PostgreSQL adapter implement the same interface. The HTTP API and scheduler propagate failed reads rather than returning fabricated zero-value finance data. The PostgreSQL adapter uses read-only transactions for reads, serializable row-locked transactions for writes and bounded retries on serialization conflicts. Source inputs and resulting scan evidence commit together as one JSONB snapshot.

## Setup for a disposable private test workspace

**Prerequisites:** PostgreSQL 15+, Go 1.23+, `psql`, and a dedicated development database. Do **not** use real customer financial data to try this feature.

1. Apply migrations as a **separate privileged migration owner**:

```bash
DATABASE_URL='postgresql://migration_owner:***@localhost:5432/revenueleak_dev?sslmode=require' bash db/migrate.sh
```

2. Provision one organization UUID with the migration owner (in your preferred SQL administration tool). Forced RLS requires transaction-local organization context for a non-superuser migration owner:

```sql
BEGIN;
SELECT set_config('app.organization_id', 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa', true);
INSERT INTO revenueleak.organizations (id, slug, display_name)
VALUES ('aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa', 'pilot-workspace', 'Private pilot');
COMMIT;
```

3. Create a **different runtime role** using your database administration process and a strong secret stored outside source control. It must not be a schema owner, superuser or `BYPASSRLS` role. Grant only the permissions needed by this **private JSONB workspace adapter**:

```sql
GRANT USAGE ON SCHEMA revenueleak TO revenueleak_runtime;
GRANT EXECUTE ON FUNCTION revenueleak.current_organization() TO revenueleak_runtime;
GRANT SELECT ON revenueleak.organizations TO revenueleak_runtime;
GRANT SELECT, INSERT, UPDATE ON revenueleak.workspace_documents TO revenueleak_runtime;
```

4. Use the runtime role's DSN and a separate, sufficiently long session secret. **Do not** enable `DEMO_MODE` with PostgreSQL.

```bash
export DATABASE_URL='postgresql://revenueleak_runtime:***@localhost:5432/revenueleak_dev?sslmode=require'
export WORKSPACE_ID='aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa'
export ADMIN_PASSWORD='your-unique-long-admin-password'
export SESSION_SECRET='a-different-high-entropy-secret-at-least-32-characters'
export ADDR='127.0.0.1:8080'
go run ./cmd/server
```

TLS mode `require` encrypts transport but does not fully verify server identity. For managed production-like databases, prefer `sslmode=verify-full` with a trusted CA, and verify that the hostname matches. `sslmode=disable` is acceptable only for disposable loopback CI/dev fixtures, never across untrusted networks.

The server rejects invalid workspace UUIDs, absent organizations, and privileged database logins. It logs **storage type**, not the database DSN. Session-cookie and reverse-proxy security limitations from `SECURITY.md` still apply.

**Existing JSON data is not migrated automatically.** PostgreSQL mode starts empty unless you explicitly use the guarded one-time migration command described below. Do not point a live instance at the new database and assume existing findings/history have moved.

## One-time import from an existing JSON pilot

A guarded migration CLI now transfers the existing pilot snapshot to a **previously provisioned, unused** PostgreSQL private workspace. It preserves imported contracts, usage, invoices, credits, mappings, findings, investigation notes, scan sequence, coverage checks and retained run evidence. It does **not** transform the data into the normalized relational tables; the private PostgreSQL adapter continues using `workspace_documents` JSONB.

**Safety procedure:**

1. Stop the original JSON-backed application and any importer/scheduler using it. Do not change the source during migration. Preserve a separate offline backup of the JSON file and a PostgreSQL database backup before any cutover.
2. Create a fresh, dedicated PostgreSQL organization using the privileged migration account, run `db/migrate.sh`, and provision an unprivileged runtime role following the steps above. Ensure its workspace has never been initialized by the application.
3. Check the source locally before connecting to the destination:

```bash
go run ./cmd/migrate-pilot -source ./data/revenueleak.json
```

4. Review the printed source SHA-256 digest, source record counts, scan sequence and retained history. The dry run does **not** connect to PostgreSQL or write anything.
5. With `DATABASE_URL` and `WORKSPACE_ID` pointing to the *new, dedicated target* and only after taking backups, perform a one-time import:

```bash
DATABASE_URL='postgresql://revenueleak_runtime:***@127.0.0.1:5432/revenueleak_dev?sslmode=verify-full' \
WORKSPACE_ID='aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa' \
go run ./cmd/migrate-pilot -source ./data/revenueleak.json -apply
```

6. The command commits the snapshot in one serializable transaction, independently reads it back and compares a canonical JSON SHA-256 digest. It does **not** delete or modify the source file. Start the private PostgreSQL-backed server only after successful verification and review the historical case list and saved scans.

**Fail-closed behavior:** Import refuses missing/invalid input, symbolic links, unknown JSON fields, multiple JSON documents, duplicate run IDs, excessive history, sources over 64 MiB, unprovisioned workspaces, privileged DB runtime users, or a destination with **any prior committed initialization**. Even a previously initialized workspace that is currently empty cannot be overwritten. Never force the import by manually deleting the target row or bypassing the version check. If a post-commit read-back fails, **stop** and investigate the target before any retry; the import may already have committed. Use your backups and a newly provisioned organization for a clean recovery.

**Limitations:** This is an assisted *one-time private-pilot cutover*, not zero-downtime migration, disaster-recovery validation or a bidirectional synchronization mechanism. The process cannot guarantee that the source was not externally modified after it was read; stop all writers. It preserves the current pilot history cap of 25 runs and inherits the financial-rule limitations described in `README.md`. Do not use it to combine customers or enable shared-tenant hosting.

## Migration runner and CI guarantees

The `db/migrate.sh` script uses a private `schema_migrations` ledger with SHA-256 checksums; each SQL migration and its ledger insert occur in one PostgreSQL transaction. Identical reruns are skipped; changed previously applied files are rejected. No destructive automatic down migration is provided—back up and test restore before an upgrade.

CI provisions a disposable PostgreSQL 16 database and verifies:
- Forced RLS on the 0001 tenant tables and an unavailable/no-tenant default deny.
- Two organizations can use overlapping provider IDs without reading or modifying each other's records.
- Cross-organization findings-to-run references are blocked by composite foreign keys.
- SQL migration idempotent reruns and checksum drift rejection.
- The live Go PostgreSQL repository (with an unprivileged role) separates alpha/beta workspace documents, persists data across repository instances, rolls back failed mutations, rejects unprovisioned workspaces and surfaces closed-connection failures.

## Security and product limitations

**Do not deploy this as a shared-customer SaaS yet.** The process is configured for exactly one trusted organization ID at startup, and the app uses one administrator credential. There is no org-picker, user membership verification, OIDC, RBAC, SCIM, per-user approval attribution, tenant-aware rate limiting, encrypted connector vault, JSON-to-relational migration, scheduled distributed jobs or audited disaster recovery.

`app.organization_id` is a transaction-local PostgreSQL setting, chosen from server configuration here. A future multi-tenant service must derive it from a **verified** session and organization membership for each request, never from an untrusted header or browser parameter. PostgreSQL GUC settings are not authentication; database SQL injection can weaken this boundary. Use parameterized SQL, strict runtime privileges, and independent tenant authorization checks.

The JSONB row per organization is intentionally transitional. It retains the pilot's 25-run retention cap and single-document write concurrency cost. Normalized source tables, durable evidence lifecycle, reconciliation fixtures and audit assurances must be implemented before demanding multi-company workloads.

## Next development gates

1. Build normalized, org-scoped repositories for contracts, invoices, usage, mappings, findings and immutable evidence. The one-time JSON-to-PostgreSQL JSONB workspace cutover exists, but **JSONB-to-normalized-relational** conversion does not.
2. Add OIDC logins, membership/RBAC checks and server-level two-organization negative tests for every endpoint (especially exports, investigations and connector configuration).
3. Add durable ingestion jobs, source checkpointing, idempotent API retries and finance approval guardrails.
4. Test production-like backup/restore, concurrency, load, PII minimization, monitoring, and an independent security review.

See [release gate issue #5](https://github.com/savisaluwadana/RevenueLeak-OS/issues/5).
