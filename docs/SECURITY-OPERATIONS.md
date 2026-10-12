# Security, privacy, deployment and operations

**Security status:** development/pilot. No independent penetration test, SOC 2 examination, enterprise compliance claim, user identity/role model, production privacy framework, automated backup vault or guaranteed recovery SLA. This document describes safe pilot operation **and** mandatory gates for a commercial shared SaaS.

## 1. Security boundaries

| Boundary | Current control | Residual risk / required mitigation |
| --- | --- | --- |
| User login | Single admin environment password, signed 24-hour HttpOnly SameSite=Strict cookie | No individual attribution, OIDC/MFA/RBAC, server-side session revocation, multiple users |
| Browser requests | Same-origin JS, HMAC-derived CSRF for authenticated mutations, CSP and security headers | Browser risks, proxy TLS termination awareness, unreviewed frontend dependency security |
| File uploads | 2 MiB max multipart, strict CSV validation, reject malformed financial rows | Source trust, schema attacks, late data, memory/resource abuse and sensitive CSV handling |
| Secrets | Env-held session and optional Stripe restricted read-only key | No KMS, encrypted vault, delegated OAuth or automated rotation |
| JSON local store | Mutex, atomic rename, `0600`, fsync | No at-rest application encryption, no multi-process lock, no tamper-evident history |
| PostgreSQL private mode | Forced RLS + static trusted WORKSPACE_ID, transaction-local context, non-owner runtime role | Not request-level multi-tenant auth; SQL injection remains threat; JSONB full-document contention |
| Billing input | Optional restricted Stripe reader and CSV | No signature-verified provider webhooks, complete provider source guarantees, duplicate-source attestation |
| Finance arithmetic | Deterministic int64 cents, source evidence, explicit credits and skipped periods | Limited contract rules; absent source data cannot prove correct or recoverable amounts |
| Finance recovery events | Authenticated/admin operator assertion, CSRF, key dedupe, evidence fingerprint and bound checks | Not bank-verified, not dual-controlled, mutable underlying document; needs independent approval and receipts |
| Backups | Manual mode-0600 no-clobber JSON; fresh-org restore integration test | Plaintext exports, no managed PITR, encryption/key custody or production RPO/RTO |
| Activity | Last 500 event strings | No authenticated actor, independent audit store, cryptographic anchoring or retention policy |

**Never treat a passing unit test as security certification.**

## 2. Deployment configurations

### A. Local sandbox

```bash
DEMO_MODE=true ADDR=127.0.0.1:8080 go run ./cmd/server
```

Synthetic data only; passwordless demo must not be accessible over a public interface or reverse-proxied to users. The demo data may be reset.

### B. Single-admin private JSON pilot

Set `ADMIN_PASSWORD`, a separate high-entropy `SESSION_SECRET`, `DATA_FILE`, `WEB_DIR`, `ADDR`. Bind to localhost or enforce network/VPN access. Do not run multiple Go instances against one JSON file. Protect the storage volume and offline backup. Do not expose uploads or exports to unrelated third parties.

### C. Experimental private PostgreSQL

Run `db/migrate.sh` using the migration owner on a dedicated PostgreSQL 15+ database. Provision one organization. Grant a separate role only schema usage, organization SELECT and workspace document SELECT/INSERT/UPDATE. Set `DATABASE_URL`, `WORKSPACE_ID`, `ADMIN_PASSWORD`, `SESSION_SECRET` (at least 32 characters). The runtime refuses privileged database roles and DEMO_MODE. Prefer verified TLS (server hostname/CA), never plaintext DB credentials in Git/logs.

**Shared SaaS hosting is not supported.** A single server's static WORKSPACE_ID is not equivalent to verified membership and per-request authorization. Do not enable host-based tenant switching or trust a client-provided org header. See `POSTGRESQL-FOUNDATION.md`.

### Reverse proxy and cookies

Terminate TLS with a trusted proxy that forces HTTPS, sets HSTS at the TLS edge, limits upload sizes and request rates, prevents access to nonpublic endpoints, and validates upstream host headers. Cookies are currently marked `Secure` when the Go process **directly** receives TLS; proxied HTTPS needs explicit validation/hardening before remote public deployment. Do not trust arbitrary `X-Forwarded-Proto` from clients. Verify session revocation and refresh architecture before production.

## 3. Threat model summary

| Threat | Scenario | Required production control |
| --- | --- | --- |
| Cross-tenant data leak | Wrong organization in SQL, export, task or AI tool | Verified OIDC principal + membership, per-route RBAC, RLS, 2-tenant negative tests on every route |
| Privileged DB bypass | App uses migration-owner or BYPASSRLS credentials | Separate roles, forced RLS, automated least-privilege and ownership assertions |
| Invoice fraud | Misleading or duplicated independent billing source | Provider-scoped dedupe, reconciliation checks, reviewed identifiers and credit notes |
| False debt claim | Missing usage/invoices interpreted as collectible | Nonmonetary quality warnings, signed-off contract rules, finance review, no auto-collections |
| Agent prompt injection | Invoice/contract tells LLM to disclose data or issue payment | Read-only tools, source content treated as untrusted, policy gating, human authorization |
| Key theft | Stripe/warehouse token in env, logs or support export | Managed KMS, scope/read-only, rotation, egress restrictions, leak detection |
| Replay | Multiple webhooks/import jobs create duplicate charges | Signature verification, idempotency, source revision rules, durable cursor/outbox |
| Tampering | Operator rewrites mutable JSON audit or recovered amount | Append-only external records, authenticated actor, maker-checker and independent verification |
| Ransomware/data loss | DB corruption, encrypted disks, lost data | Managed encrypted backups, immutable off-site copies, PITR, recovery drills and RPO/RTO |
| Privacy breach | Contract/customer PII exported or retained forever | Data minimization, retention/deletion, DPA, regional controls and access logging |
| Availability failure | Long Stripe sync, disk full, exhausted job queue | Background workers, budgets/timeouts, alerts, quotas and backpressure |

## 4. Privacy/data governance plan (TARGET, not implemented)

- Document controller/processor roles and supported jurisdictions, customer contractual basis and operator consent.
- Classify PII and financial data fields; minimize imported names, identifiers and notes.
- Define tenant-specific retention periods for source events, snapshots, review/approval records, backups and request logs.
- Implement erase/export requests, suspension and retention exceptions with legal hold handling; ensure evidence integrity rules are documented.
- Encrypt at rest via managed DB/volume and keys, plus independent encrypted backup vault and managed key rotation.
- Maintain internal restricted access logs, secrets redaction, breach-response process and subcontractor transparency.
- Ensure agents are opt-in with DPA, regional processing controls, no secrets in model prompts, explicit retention settings and redaction.
- Threat model organization deletion and payment-provider disconnect; reconcile snapshot retention with finance obligations.

## 5. Operations runbooks

### Start / health

1. Validate environment and secrets, database access, migrations, correct WEB_DIR and restricted HTTP bind.
2. Run `go test ./...` plus `go vet ./...` and the browser smoke tests before release.
3. Verify `GET /healthz` (process only), authenticated `GET /api/me`, `GET /api/overview`, `GET /api/scans` and source upload permissions.
4. Check last scan, source timestamp and freshness manually: **no live ingestion freshness SLA currently exists**.
5. Run a known synthetic fixture and inspect evidence before introducing approved pilot files.

### JSON-to-private-PostgreSQL migration

Stop ALL writers, take source/database backups, inspect source using `cmd/migrate-pilot` in dry-run, provision a fresh workspace, apply with explicit `-apply`, compare post-import digest, review all financial summaries and recent evidence, then cut over. Do not retry after a read-back failure without checking whether the transaction committed.

### Manual backup and test restore

```bash
# DATABASE_URL / WORKSPACE_ID must reference ONE private organization
go run ./cmd/backup-workspace -output /restricted/backups/workspace-unique-YYYYMMDD.json
# Encrypt and store off-site using an approved tool and vault.
# Provision a NEW organization and set target DATABASE_URL / WORKSPACE_ID
go run ./cmd/migrate-pilot -source /restricted/backups/workspace-unique-YYYYMMDD.json
go run ./cmd/migrate-pilot -source /restricted/backups/workspace-unique-YYYYMMDD.json -apply
```

Protect the JSON file during transfer and dispose of decrypted temporary copies securely. This covers a limited **workspace document** restore test, **not** full managed PostgreSQL database restore, WAL replay or guaranteed RPO/RTO.

### Suspected financial error

Do not bill or contact the customer automatically. Quarantine the affected findings, identify source run/rules version, check mapping and usage completeness, reproduce with a finance-signed fixture, capture before/after evidence, reopen earlier approvals if evidence changed, notify impacted pilot operators and document corrections.

### Suspected security incident

Rotate affected keys and sessions under admin-approved procedures, isolate connectors, preserve encrypted evidence, restrict data access and review logs; involve qualified security/privacy leadership and follow applicable legal notification and processor agreements. Do not publish customer record contents in GitHub issues.

## 6. Monitoring and SLO design (TARGET)

Target metrics: API p50/p95/p99 latency and errors by route, tenant-scoped DB read/write failures, lost/corrupt snapshot checks, source freshness and watermark, ingest retries/DLQ, scan lag, skipped rules per reason, contradictory mappings, potential/confirmed/verified-cash precision, auth failures, backup age, restore drill outcome and provider rate-limit pressure. Target traces include opaque tenant ID and job/case IDs only; do not log raw customer content, sensitive notes or credentials.

Draft SLO candidates require customer validation, service ownership and measurements. Do not promise numbers that have never been met or measured.

## 7. Release gates before any multi-customer production launch

- [ ] OIDC/MFA, server-side session revocation, organization membership and role-by-route enforcement with cross-tenant negative tests.
- [ ] Normalized transactional data and tenant-scoped audit/evidence operations; migration proven with financial fixtures.
- [ ] KMS/vault-backed read-only connector keys and rotation/consent.
- [ ] Durable ingestion and scheduled reconciliation with retries, DLQ, cursor/watermarks and completeness evidence.
- [ ] Finance-approved contract rules and reconciliation outputs, including actual source disagreements and false positive metrics.
- [ ] Maker/checker reviews and independently verified receipt/correction ledger.
- [ ] Confidentiality, DPA, retention/deletion controls, backup encryption, managed PITR and isolated restore drills with measured targets.
- [ ] Load testing, dependency/security scans, penetration review, incident response exercises and release signoff.
- [ ] Pilot customer acceptance with documented data owner and signed DPA/consent.

For the current pilot-specific limitations read `../SECURITY.md`. For engineering tasks and milestones see `IMPLEMENTATION-PLAN.md`.
