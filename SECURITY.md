# Security, privacy and operational readiness

## Default posture

- Passwordless demo mode is **allowed only on localhost**; do not reverse proxy an unauthenticated demo.
- Private mode requires `ADMIN_PASSWORD` in the environment. Use a strong independent `SESSION_SECRET`, preserve only in a secrets manager, rotate on suspected compromise.
- Signed HttpOnly, SameSite=Strict cookies have a 24-hour expiry. Use HTTPS and a trusted TLS reverse proxy for any non-local access. Place the service behind a firewall, SSO or VPN until the production auth stack exists.
- APIs enforce a per-session CSRF token for changes; CSP limits scripts to same origin, and standard browser security headers are emitted.
- Sign-in is rate-limited per remote IP (memory-only). The reverse proxy must implement network-wide rate limits and DDoS controls.
- Stripe keys remain exclusively in server environment configuration. Use separate restricted, read-only API keys with minimum permissions, and avoid exposing the process environment in logs or diagnostic pages.
- No payments, invoices, customer emails, or refunds are created or modified by the application.

## Known pilot security gaps

1. No SSO, MFA, RBAC, multiple users, SCIM or audit attribution to individual users.
2. JSON persistence uses OS permissions but **no application-level at-rest encryption**. Encrypt the storage volume and backups; enforce OS access controls.
3. No **authenticated multi-tenant HTTP isolation**. Optional PostgreSQL mode binds a whole private server process to one static, provisioned workspace ID via transaction-local RLS. The HTTP server still uses one admin credential and has no OIDC/RBAC or per-user organization authorization. Never co-host multiple real customer workspaces in one public pilot instance.
4. Audit events are mutable alongside the data store, not an externally secured append-only log.
5. Session cookies are marked `Secure` when the Go app directly terminates TLS. With TLS offload, ensure HTTPS enforcement at the proxy and cookie security validation before exposing externally.
6. CSP permits inline *styles* to support dynamic chart rendering; scripts remain same-origin only.
7. Saved runs have a fixed 25-run retention cap, but there is no organization-specific privacy/consent policy, configurable retention, automated backup/restore or tested disaster recovery.
8. No vulnerability scanner or independent penetration test has been performed.
9. Connector synchronization may be slow on large Stripe accounts; do not run concurrent full sync jobs in a production shared instance.
10. Billing correctness depends on complete and trusted source mappings. An underbilled finding is not a payable invoice.

## Production release checklist

- [ ] Threat model, security testing, dependency scanning, secrets scanning and independent review.
- [ ] Database migration and backup/restore drills.
- [ ] Tenant and permission model with integration tests for cross-tenant access.
- [ ] Modern OIDC/SSO and MFA, secret rotation, logout session revocation.
- [ ] Observability, synthetic probes and per-tenant ingest/reconciliation SLAs.
- [ ] Encrypted keys with KMS or secrets manager and fine-grained access scopes.
- [ ] Data-processing agreement, deletion/retention and legal review.
- [ ] Full pricing and invoice lifecycle support, plus finance signoff for each rule.
- [ ] Resilience and concurrent workload tests.
- [ ] Incident response, breach notification and change approval policies.

## Reporting vulnerabilities

Please do not test this pilot against third-party customer billing data or a public production system without permission. Report security findings through a private channel to the application owner. Do not include credentials or sensitive customer data in public issues.

## Run evidence and retention

Saved runs duplicate customer contracts and financial inputs in the private JSON store. The latest 25 scans are retained; older runs are pruned during successful scans. A demo reset clears all workspace data and history. Backups and exported JSON files are separate copies and require their own retention/deletion controls. Do not commit them or attach real customer exports to public GitHub issues. Hashes are not signatures and do not prevent a filesystem administrator from rewriting evidence.

## PostgreSQL foundation security boundary

The database migrations and RLS isolation tests in `db/` support an **optional single-private-workspace adapter**; they do not protect the default JSON-backed demo and do not implement per-user tenant authorization. PostgreSQL mode rejects schema owners, superusers and `BYPASSRLS` roles, and scopes queries with an environment-configured organization UUID. A future multi-tenant service must instead derive this value from a verified organization membership. Custom PostgreSQL session settings are not themselves authentication; SQL injection and broad grants remain threats. See [PostgreSQL foundation](docs/POSTGRESQL-FOUNDATION.md).

## One-time private-workspace migration security

`cmd/migrate-pilot` defaults to read-only dry-run. Apply mode requires an unprivileged PostgreSQL runtime login and a previously provisioned organization; the source must be a regular, non-symlink JSON file, and the import refuses an initialized target to prevent overwriting existing finance records. Exported digests and source counts are not accounting certification. Run imports offline after backing up both systems; protect snapshot data and console output, avoid credentials in shell history, and never run the command against a shared-customer production environment. See `docs/POSTGRESQL-FOUNDATION.md`.

## PostgreSQL private-workspace backups

The manual `backup-workspace` CLI writes an atomically published, no-clobber, mode-`0600` JSON snapshot of a single provisioned PostgreSQL workspace. **It is not encrypted**; exported financial records and saved case evidence are sensitive. Run it only in trusted environments, use an encrypted backup vault and restricted off-site storage, and regularly test restoring into a separate empty organization using the guarded migration CLI. Backups do not replace managed PostgreSQL PITR, retention policies, key management or incident recovery plans. Never share real customer backup files in public PRs or issues.
