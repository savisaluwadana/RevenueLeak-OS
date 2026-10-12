# API reference

The JSON API is same-origin. Private workspaces require the signed `rl_session` cookie; passwordless demo access is localhost-only. Mutations require `X-CSRF-Token` from `GET /api/me`, except login. `POST /api/login` accepts `Content-Type: application/json` with `{"password":"..."}`. No endpoint creates charges or invoices.

| Method | Route | Result |
| --- | --- | --- |
| GET | /healthz | Public liveness status |
| GET | /api/me | Authentication, CSRF and capabilities |
| POST | /api/login, /api/logout | Session operations |
| GET | /api/overview, /api/findings | Current calculations and investigations |
| GET | /api/datasets, /api/audit | Current source data and activity |
| POST | /api/import/{kind} | CSV multipart upload (`file`), upsert and saved scan |
| POST | /api/scan | Scan summary including run_id and reopened_findings |
| POST | /api/findings/{id}/status | Current review status/note update |
| GET | /api/findings/export | Current findings CSV |
| POST | /api/stripe/sync | Read-only invoice sync and saved scan |
| POST | /api/demo/reset | Destructive reset, demo mode only |
| GET | /api/recoveries | Operator-entered finance assertions, separate current-evidence confirmations and recorded collections |
| POST | /api/findings/{id}/recovery | Idempotent finance confirmation / collection assertion; never transfers money |
| GET | /api/coverage | `{gaps: [...]}`; source coverage warnings only (no monetary totals) |
| GET | /api/scans | `{runs: [...], retention_limit: 25}`; summaries only |
| GET | /api/scans/{id} | `{run: {...}}`; recorded sources and results |
| GET | /api/scans/{id}/export | Download `{schema_version: "1", run: {...}}` as JSON |

`kind` is contracts, usage, billing, credits, or mappings. Source CSV templates are in `samples/`. A finding status payload is `{"status":"reviewing","note":"..."}`; allowed statuses are new, reviewing, approved, false_positive and resolved.

Saved-run routes require a private session and return `401` without it. Missing or expired runs return `404`; JSON errors have an `error` string. Run JSON responses use `Cache-Control: no-store`. Downloads include customer contracts and billing inputs. The source digest identifies the saved inputs, not financial collectibility or external certification.

## Source coverage checks

`GET /api/coverage` requires the same private session as findings, and returns a `gaps` array containing `id`, `customer_id`, `customer_name`, `period`, `metric`, `currency`, `monthly_minimum_cents`, `reason` and `explanation`. This endpoint does **not** declare any money recoverable. `GET /api/overview` includes `coverage_count` and each scan summary includes `coverage_gaps`. Saved runs include a snapshot of the coverage array used in that run.

Only effective-dated contracts with positive **monthly minima** are examined, for the last 12 eligible closed months after the existing seven-day grace. A warning is produced when neither independent usage nor finalized paid/open billing exists for that customer/metric/month. One present source does not guarantee completeness, and draft/void billing is not considered finalized.

## Automatic reconciliation

Set `AUTO_SCAN_INTERVAL` to a Go duration of at least `1h` (e.g. `24h`) in the server process environment. No extra API endpoint is added: the schedule runs server-side with the same 7-day closed-period grace and `trigger=scheduled` in saved history. It skips another scan for unchanged source fingerprints and rule versions within the same UTC day. Manual scans/imports still function. There is no connector refresh, notification or distributed scheduler in this pilot.

## Provider customer identity mapping

Import `mappings` with a CSV containing `source,external_customer_id,customer_id`. Allowed source values: `stripe`, `billing_csv`, `usage_csv`, `credits_csv`. Mapping targets refer to the canonical `customer_id` in contract records. The source/external ID pair is an upsert key; contradictory rows in one uploaded CSV are rejected. Mapping imports trigger reconciliation and save a run like other source changes.

`GET /api/datasets` includes the `mappings` array. Saved run inputs include a snapshot of mappings; the source fingerprint includes mapping changes. Raw usage, credits and invoice source records keep their external IDs. Unknown billing sources (including synthetic demo data) cannot inherit CSV aliases. Imports are authenticated and CSRF-protected.

**Accuracy warning:** Explicit mappings are asserted by the importing operator, not inferred or verified by the software. Many-to-one mappings can legitimately combine multiple billing customers but require independent finance verification to avoid duplicate billing totals. Full ingestion checks, provider customer-identity attestation, and tenant-level access policies remain production work.

## Storage backends

The HTTP API contract is unchanged. The default server is the local JSON pilot. Opt-in PostgreSQL mode (`DATABASE_URL` plus `WORKSPACE_ID`, `DEMO_MODE=false`, strong `ADMIN_PASSWORD` and independent `SESSION_SECRET`) stores **one private organization's** pilot snapshot in the transactionally updated `revenueleak.workspace_documents` table. It is not an OIDC multi-user SaaS API and does not authorize access to different organizations per HTTP session. Read failures return HTTP 500 instead of artificial empty results; saved-run reads distinguish database failure (500) from missing/expired history (404). See [PostgreSQL setup](POSTGRESQL-FOUNDATION.md).

## Operator-entered recovery ledger (pilot)

The recovery ledger is **not a cash settlement system**. It is a per-finding append-only-in-application record of what the single logged-in workspace admin **states** has been finance-confirmed or collected. It never creates a charge, invoice, refund, email or provider transaction, and it does not independently query or verify payments. The underlying JSON/JSONB document remains mutable by authorized storage administrators; this is **not** a tamper-evident accounting ledger.

**GET `/api/recoveries`** requires the signed private session and returns:

```json
{
  "events": [
    {
      "id": "rec_...",
      "finding_id": "leak_...",
      "customer_id": "c1",
      "currency": "USD",
      "evidence_digest": "64-character-SHA-256-source-fingerprint",
      "action": "confirmed",
      "amount_cents": 2500,
      "note": "Finance checked signed rate card and invoice",
      "at": "2026-10-12T01:00:00Z"
    }
  ],
  "totals_by_currency": {
    "USD": {
      "confirmed_current_cents": 2500,
      "collected_recorded_cents": 0,
      "needs_review_count": 0
    }
  },
  "disclaimer": "Operator-entered records only..."
}
```

**POST `/api/findings/{id}/recovery`** requires the signed session, `X-CSRF-Token`, `Content-Type: application/json` and:

```json
{
  "idempotency_key": "review-case-unique-20261012",
  "evidence_digest": "the current 64-character finding.evidence_digest",
  "action": "confirmed",
  "amount_cents": 2500,
  "note": "Verified current signed terms and original billing lines"
}
```

`action` is exactly `confirmed` or `collected`. Amount is a positive integer in the finding's minor currency units. `idempotency_key` must be 8–128 alphanumeric, underscore, hyphen, or dot characters; reuse **the same key and payload** to safely retry a request. Same key with changed payload returns 409. Notes must contain at least eight non-whitespace characters and at most 1500 bytes/characters according to the server's string length check. The current finding must be Approved or Resolved and the `evidence_digest` must match exactly.

A cumulative **confirmation** may not exceed that finding's current potential discrepancy. A cumulative **collection recorded** may not exceed the confirmation total for that same evidence version. Reconciliation does not consume these entries as invoice lines and does not make them counted cash; after input changes old confirmation is excluded from current-evidence totals, and old collection assertion remains visible as history. USD/EUR/GBP totals are kept separate. Pilot ledger is capped at 10,000 records.

Responses: `200` with `entry` on creation or exact idempotent retry; `400` for invalid amount/status/notes or over-cap; `401` unauthenticated, `403` missing/invalid CSRF; `404` missing finding; `409` stale evidence or key conflict; `500` persistence failure. The API does not support editing/deleting recovery records. Any correction/reversal workflow with independent finance signoff must be added before production.

### Remaining API work (NOT currently exposed)

- `/api/organizations`, invitations, memberships, OIDC callback/session revocation, role grants.
- Tenant-identified connector secrets, ingest jobs, status, provider cursors and webhook receive endpoints.
- Versioned contracts and advanced pricing rule management.
- Case assignment, workflow due dates, second-person approvals, correction and provider-verified collection records.
- AI investigation/evidence retrieval, LLM budgets/evaluations and pre-invoice simulation.
- SaaS plans/entitlements, account subscription billing and commercial administration.

See [complete API-adjacent workflows](WORKFLOWS.md), [implementation plan](IMPLEMENTATION-PLAN.md), [architecture](ARCHITECTURE-REFERENCE.md) and [documentation index](README.md).
