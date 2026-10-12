# Testing and financial correctness strategy

**Existing test coverage:** GitHub Actions runs `gofmt -l`, `go vet`, `go test -race ./...`, a Go executable launcher smoke, Node syntax, Playwright Chromium desktop/mobile smoke, Docker image build and Compose syntax, SQL migration checksum/drift tests, forced-RLS two-organization SQL isolation tests and opt-in PostgreSQL integration tests (private storage, import, backup/restore, role guard).

**Not tested or not proven by the above:** production bank reconciliation, full customer billing cycles, complete contract terms, shared-app tenant authorization, security certification, load/SLA/recovery targets, agent risk evaluations and enterprise provider integrations. Do not confuse unit success with finance signoff.

## 1. Run the existing checks

```bash
go mod download
gofmt -l .
go vet ./...
go test -race ./...
go build -o /tmp/revenueleak ./cmd/server
node --check web/app.js
npm ci
npx playwright install chromium
npm run test:browser
docker build -t revenueleak:test .
```

To test PostgreSQL locally, provision a disposable PostgreSQL 16 database with migrations, two or more org fixtures and a non-owner runtime role. Set `POSTGRES_TEST_URL` and run `go test -race -tags=postgres_integration ./internal/pgstore ./cmd/backup-workspace`. The CI workflow is the normative fixture setup; never copy its sample credentials into a real environment.

## 2. Financial golden fixtures: acceptance matrix

Build repeatable signed-off input/output fixtures that include an explicit expected amount, skip reason and context evidence. Proposed cases:

| ID | Case | Expected contract behavior | Current support |
| --- | --- | --- | --- |
| F001 | $0.10/unit, 100 included, 300 used, 1500 cents invoiced | Expected 2000 cents; variance 500 | Built/tested |
| F002 | Positive monthly minimum, no finalized invoice but usage present | Apply max(base, minimum), explicit approved credits | Built/tested |
| F003 | Period not past month-end + 7-day grace | No monetary finding yet | Built/tested |
| F004 | Draft/void invoice only | Exclude from finalized sum | Built/tested |
| F005 | Contract missing or FX mismatch | Skip; never invent contract or convert currency | Built/tested |
| F006 | No usage AND no billing for active minimum contract | Non-monetary source quality warning | Built/tested |
| F007 | Source customer alias | Exact source-scoped canonical mapping without rewriting raw ID | Built/tested |
| F008 | Reimport same usage/invoice IDs | Idempotent single source key upsert | Built/tested |
| F009 | Evidence changes after case approval | Reopen, preserve old notes/run snapshot | Built/tested |
| F010 | Recovery confirmation/collection recorded | Positive approved current evidence, bounded values, idempotency | Built/tested as operator assertion |
| F011 | Tiered/graduated prices | Correct tier boundary, rounding and amendments | TARGET |
| F012 | Prepaid/annual contract minimum, rollover | Proper amortization and commitment balance | TARGET |
| F013 | Refunds/taxes/write-offs/discounts | Contract-specific net accounting semantics | TARGET |
| F014 | Late/backfilled independent usage | Source freshness blocks premature confidence; replay adjusts findings | TARGET |
| F015 | Same invoice from CSV and provider | Cross-provider duplicate identified and handled | TARGET |
| F016 | Bank/payment-provider verified receipt | Prove posted receipt and avoid double counting | TARGET |
| F017 | Complex currency and FX | Currency-specific source and legal entity rules | TARGET; never mix currencies |

For every added pricing rule: provide fixtures with both exact expected cents and why/which source IDs generated that output; require finance/business signoff before calling a result recoverable.

## 3. Data integrity testing

- CSV missing columns, duplicated IDs, invalid dates, negative quantities or money, too many bytes, malicious formula strings, unknown source mappings and row reorder.
- Conflicting overlapping contract versions, no contract, empty source, altered exchange/currency, closed-period grace and exception paths.
- Durable transaction rollback on parsing/reconcile/storage failure; do not save partial input or partial evidence.
- Restart persistence and repeatable saved run input/result digests.
- Immutable-in-application evidence snapshots despite later source edit, review and status changes.
- Bounded history behavior; explicit retention rather than claims of unlimited audit.
- Recovery assertions: cannot confirm more than variance; cannot collect more than approved/confirmed amount; no automatic payments; stale evidence invalidation; repeat API request with same idempotency key; conflicting reuse; operator note limits; currency-specific totals; no current cash proof.
- DB adapter negative cases: missing organization, owner/BYPASSRLS login, closed connection, serialization conflict, identical external IDs in independent orgs, rejected org crossing, backup source/target hash equality.

## 4. Multi-tenant access negative test matrix — TARGET

The current private-workspace tests **do not satisfy** this API-level matrix. Build a set of at least two authenticated users and two organizations plus independent roles. Attempt every route with no identity, invalid session, role without permission, valid identity but wrong org and guessed resource IDs.

| Operation | Viewer | Analyst | Finance approver | Org admin | Cross-org |
| --- | --- | --- | --- | --- | --- |
| View own findings and scans | Read | Read | Read | Read | Denied |
| Download own detailed financial evidence | Scoped read | Scoped read | Scoped read | Read | Denied |
| Import source / change mapping | Denied | Conditional | Denied | Write | Denied |
| Run/manual replay | Denied | Write | Conditional | Write | Denied |
| Review case status | Denied | Draft review | Approve only if not maker | Write subject to policy | Denied |
| Confirm recovery | Denied | Denied | Independent approver | Policy guarded | Denied |
| Record independently verified collection | Denied | Denied | Policy guarded | Policy guarded | Denied |
| Change membership / role | Denied | Denied | Denied | Authorized admin | Denied |
| Rotate connectors / KMS secrets | Denied | Denied | Denied | Authorized admin | Denied |
| Delete/export org data | Denied | Denied | Denied | Policy/consent guarded | Denied |

This matrix is a proposed RBAC policy, **not implemented enforcement**. Finance roles and scopes must be refined with design partners, legal and internal security owners.

## 5. End-to-end scenarios

### Current pilot test scenarios

1. First-run local synthetic demo; no exposed non-local unauthenticated access.
2. Admin login, CSRF rejection for mutations without token, unauthorized read rejection and private errors.
3. Import all sample CSVs, check data counts and currency-separated calculations.
4. Review case, change evidence, rerun, verify review invalidation and preserved historical snapshot.
5. Add confirmed + collected assertions, reload and compare; altered evidence drops confirmation from current total but retains historical operator collection entry.
6. Run and inspect `/api/scans/{id}/export`; verify no customer data on unauthorized response.
7. Migrate stopped JSON workspace into unused PostgreSQL org; byte-equivalent canonical hash on read-back; verify no overwrite on initialized target.
8. Backup PostgreSQL workspace to restricted file and restore to distinct empty org; compare canonical hash and notes.
9. Restart application and repeat queries. With PG disconnected, private reads must fail as 500, not show an empty financial dashboard.

### Production readiness tests — TARGET

- Two tenants concurrently importing a shared external ID/metric, scanning, exporting and approving; no cross-org exposure.
- Source backfill/retry dedupe, provider webhook invalid signature and replay window, credential rotation/revocation and DLQ drain.
- Concurrent reconciliation and schema migrations, no double-issued invoices, no stale approvals.
- Zero-trust agent: crafted instructions in invoice/contract must never trigger cross-org data access, sensitive credential exfiltration or actions.
- Full managed database PITR recovery to a clean environment, with independent backup encryption/key recovery, RPO/RTO measurements.
- Load test against expected ICP volume, with p95/p99 latency and provider rate-limit pressure recorded.
- Privacy delete/export, retention and backup deletion across every configured data store.
- Static, dynamic and dependency security testing followed by independent review and documented remediation.

## 6. Release evidence required

For each release attach: source commit SHA, CI results, migration/recovery signoff, diff of supported financial rules, new/changed golden fixtures, secure-config smoke, privacy impact notes, documented limitations and rollback plan. A production readiness statement must identify the person/entity authorizing it, date and exercised scope.

For detailed architecture/roles, see `ARCHITECTURE-REFERENCE.md`; for roadmap milestones see `IMPLEMENTATION-PLAN.md`.
