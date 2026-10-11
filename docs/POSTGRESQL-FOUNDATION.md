# PostgreSQL tenant foundation (schema-only milestone)

**Status:** PostgreSQL schema and migration tools are implemented and tested against a disposable PostgreSQL 16 instance in CI. The running RevenueLeak-OS HTTP API **still uses the single-workspace local JSON store**. This milestone is not a production database switch, not login/RBAC, and not an authorization-complete multi-tenant release.

## Scope

`db/migrations/0001_tenant_foundation.sql` defines:

| Table | Purpose |
| --- | --- |
| `organizations` | Customer workspace identities |
| `memberships` | External OIDC issuer/subject and intended workspace role |
| `contracts` | Effective-dated, versioned-per-start-month commercial prices |
| `customer_mappings` | Provider-specific external IDs to canonical contract IDs |
| `usage_records` | Independent, nonnegative metered usage with per-provider record IDs |
| `invoice_lines` | Provider-scoped invoice line identifiers and finalized/draft status |
| `credits` | Explicit credits and reasons |
| `reconciliation_runs` | Source digest, rules version, evidence JSON and result summary |
| `findings` | Evidence-backed monetary discrepancy cases |
| `approval_records` | A proposed maker/checker trail linked to case and membership |
| `ingest_jobs` | Idempotency key, provider, retry metadata, state |
| `audit_events` | Organization-scoped activity history |

Every tenant-owned table has an `organization_id` column, enforced row-level security (RLS) and a forced-RLS flag. Composite organization-qualified foreign keys are used for cross-table financial links, so an organization's finding cannot link to another organization's saved reconciliation run or approval actor. Provider record keys are scoped by organization and source.

**Important:** This is a SQL foundation, not proof of customer-grade transaction integrity. There is no Postgres adapter, no application organization membership verification, no KMS secret handling, no API permission enforcement, no live connectors or durable worker yet.

## Run migrations on a disposable database

You need PostgreSQL **15 or later**, a local `psql` client, and a database role authorized to create schemas/tables/policies. Use a fresh test database rather than an existing finance database.

```bash
export DATABASE_URL='postgresql://migration_owner:password@localhost:5432/revenueleak_dev?sslmode=require'
bash db/migrate.sh
```

The runner applies SQL files in filename order, records the SHA-256 checksum in `revenueleak.schema_migrations`, and runs each migration and its ledger insert inside one transaction. A re-run skips files with the same checksum and rejects files that changed after application.

**Rollback policy:** This first version does not provide automatic destructive `down` migrations. Back up and test restore before upgrades, and create forward-fix migrations for later schema changes. Treat a migration checksum mismatch as an incident to investigate, not a reason to delete the migration ledger.

CI starts a disposable PostgreSQL container and executes the migration runner twice, verifies isolation with `db/tests/tenant_isolation.sql`, and checks that altered migration content is rejected.

## Runtime security design — not yet connected

The intended application database login must be separate from the migration owner, with no superuser privileges, `BYPASSRLS`, table ownership or `CREATE` permission on `revenueleak`. Grant only the needed table actions, and prefer **append-only** permissions on audit and approval history where appropriate.

The `revenueleak.current_organization()` function reads the transaction-local database setting `app.organization_id`. Future application code must derive the organization ID **only from a validated server-side principal/membership**, and execute every query within the same transaction after setting:

```sql
BEGIN;
SELECT set_config('app.organization_id', 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa', true);
-- organization-scoped SQL here
COMMIT;
```

The third argument to `set_config` makes the setting **transaction-local**. Never set the tenant from an untrusted request header or browser query parameter. Never expose general SQL query execution to customers. PostgreSQL custom GUC values can be set by a database session: **RLS is defense in depth, not a substitute for robust authentication, authorization, safe SQL, and a non-bypassable application role.** SQL injection into a writable database session can undermine trust.

A missing organization setting returns no records under RLS. The CI isolation checks cover overlapping provider IDs in two organizations, denied cross-organization reads/updates/inserts, cross-organization foreign-key rejection, and default-deny reads without a tenant.

## Migration from the JSON pilot — not implemented

No automatic JSON-to-SQL migration or production data cutover is included. Do not point existing live customer data at the new schema and assume it has been migrated.

The next changes should be:

1. **Implemented:** an error-returning `core.Repository` interface with `Read() (Database, error)` and `Update(...) error`. HTTP endpoints and the scheduler fail closed on read errors. The JSON pilot implements this interface; the PostgreSQL adapter is not implemented yet.
2. Implement PostgreSQL transactional reads/writes with org-scoped connection/session context, strict transactions, and safe migration from a copy of JSON pilot data.
3. Implement OIDC identities and per-organization memberships before routing HTTP requests to different organizations. Add route-by-route role checks for imports, scans, reviews, exports and audit history.
4. Integration-test concurrent writers, import deduplication, durable scan history, backup/restore and two-tenant negative cases against real PostgreSQL.
5. Replace single-process scan scheduling with an idempotent distributed job queue and source-specific ingestion checkpoints.

**Release gate:** No public multi-tenant rollout until the application path—not only the SQL schema—passes cross-tenant isolation, data recovery and security reviews.
