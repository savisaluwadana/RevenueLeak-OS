# RevenueLeak OS

**A runnable, evidence-first SaaS revenue assurance pilot.** Reconcile independent usage, customer contract terms, approved credits, and finalized billing lines to identify *potential* underbilling. Designed for controlled finance investigation—not automatic debt collection.

![Status](https://img.shields.io/badge/release-0.2.0_pilot-green)

## Standalone repository

The application now lives at [savisaluwadana/RevenueLeak-OS](https://github.com/savisaluwadana/RevenueLeak-OS). See [migration notes](docs/MIGRATION.md), [architecture](docs/ARCHITECTURE.md), [roadmap](docs/ROADMAP.md), and [security](SECURITY.md).

## Quick start (Go 1.23+, PostgreSQL not required for demo)

```bash
DEMO_MODE=true go run ./cmd/server
```

Open **http://127.0.0.1:8080**. The default demo binds to localhost, seeds synthetic customers, and works without external credentials or network services after Go dependencies have been installed. Sample CSV files are available from the Data sources page. Data persists in `./data/revenueleak.json`; the **Settings → Reset demo data** button restores the sample dataset.

### Private workspace

```bash
export ADMIN_PASSWORD='replace-with-a-unique-long-password'
export SESSION_SECRET='replace-with-a-unique-random-signing-secret'
export ADDR='127.0.0.1:8080'
export DATA_FILE='./data/revenueleak.json'
go run ./cmd/server
```

Use a TLS reverse proxy for remote access, do not publicly expose the Go service, and keep server secrets out of Git. You can also run `docker compose up --build` after creating your own `.env` from `.env.example`. The Compose port is bound to localhost by default.

## Optional PostgreSQL-backed private workspace (experimental)

The Go server can now use PostgreSQL for **one provisioned, authenticated private workspace** when `DATABASE_URL` and `WORKSPACE_ID` are provided. The JSON demo remains the default, and Stripe remains optional. This uses a transactionally updated, tenant-scoped JSONB workspace document while the long-term normalized relational repository is still being developed. See [PostgreSQL setup](docs/POSTGRESQL-FOUNDATION.md) for migration, runtime role, org provisioning and testing instructions.

**Not production multi-tenancy:** the application still has only one admin password per running server and no OIDC, organization membership authorization, RBAC or routed multi-company sessions. Do not expose multiple unrelated customer organizations behind a shared HTTP instance. Migration from existing JSON pilot data is not automatic.

## Assisted JSON-to-PostgreSQL pilot migration

The new `cmd/migrate-pilot` command supports a **one-time, manually approved import** of a paused local pilot's JSON data into a *fresh*, provisioned private PostgreSQL workspace. It defaults to dry-run, preserves the source and history, refuses to overwrite an initialized workspace, and verifies a post-import SHA-256 fingerprint.

```bash
go run ./cmd/migrate-pilot -source ./data/revenueleak.json
# After backups and setting DATABASE_URL + WORKSPACE_ID:
go run ./cmd/migrate-pilot -source ./data/revenueleak.json -apply
```

Read the [complete migration and safety procedure](docs/POSTGRESQL-FOUNDATION.md#one-time-import-from-an-existing-json-pilot) **before applying**. This still imports into PostgreSQL **JSONB private-workspace storage**, not the future normalized relational data model, and does not implement production multi-tenancy.

## PostgreSQL tenant schema (development foundation only)

The repository contains a versioned PostgreSQL schema, a checksum-verified migration runner, and CI tests for row-level security, cross-tenant foreign keys and an opt-in private PostgreSQL repository. See [PostgreSQL foundation](docs/POSTGRESQL-FOUNDATION.md).

**The default server uses the local JSON workspace.** Opt-in PostgreSQL mode persists a single private workspace; multi-user identity, RBAC and production tenant authorization remain unimplemented.

## Canonical customer ID mappings (pilot feature)

Import [mappings.csv](samples/mappings.csv) on the **Data sources** page when different systems identify the same customer differently. Each mapping uses `source,external_customer_id,customer_id`: `source` must be `stripe`, `billing_csv`, `usage_csv`, or `credits_csv`; `customer_id` is the canonical ID in Contracts. The source field prevents accidentally treating identical IDs from unrelated providers as the same customer. Mappings are upserted by source plus external ID, and invalid/conflicting CSV rows are rejected.

The application retains the **original source IDs** in raw imported records and saved evidence, with a snapshot of the mapping rules used for each scan. Changing mappings triggers recalculation, and may reopen or replace cases. Correctness still requires a finance operator to verify mappings and source completeness; this is not a fuzzy/entity-resolution AI service. A wrong mapping can silently change totals, so use synthetic or reviewed pilot data only.

## Opt-in automatic reconciliation (pilot feature)

By default reconciliation is manual/on-import. Set `AUTO_SCAN_INTERVAL=24h` in the server environment to enable the built-in periodic closed-period audit (minimum 1h). The service checks at startup and at the configured interval, saves an audit run only when the UTC day, source fingerprint or rule version changes, and stops gracefully on SIGINT/SIGTERM. This does **not** ingest new external data, guarantee data freshness, or replace distributed job scheduling for production. Logs report failures; it does not send alerts yet. A manual CSV import continues to trigger its own reconciliation. See [development guide](docs/DEVELOPMENT.md).

## Source coverage checks (pilot feature)

The **Coverage checks** page checks the latest 12 eligible closed months (after the existing 7-day grace) for active monthly-minimum contract periods with **neither independent usage nor finalized invoice data**. Checks are derived from imported effective-dated contracts, and are updated on every scan/import. They have a separate authenticated `GET /api/coverage` endpoint and appear in saved scan evidence.

**These are data-quality warnings, not financial claims.** The contractual minimum is displayed for context only; checks do **not** increase the potential discrepancy total or create recoverable revenue findings. A gap can mean missing imports, a paused/cancelled contract not yet reflected in source data, special billing arrangements, credits or an actual omitted invoice. Verify all these with finance. Paid/open invoice lines or independent usage close this specific *both-sources-missing* check; draft/void invoices do not. The check does not yet prove complete usage or invoice data when *some* records exist.

Stripe remains optional. CSV imports and the core engine work without Stripe credentials. This does not implement distributed job scheduling, production multi-tenancy or a customer-ready billing integration marketplace.

## Saved reconciliation runs (v0.2)

Every manual scan, CSV import, Stripe sync and demo seed now saves its inputs, calculated findings, rule version and SHA-256 input fingerprint in the same datastore transaction. Open **Saved runs → Inspect** to review an earlier result or download an evidence JSON bundle. The latest **25 runs** are retained; export important runs before they expire. A later source correction or review edit does not rewrite an earlier run. Scans that produce no findings are saved too.

Reviewed cases reopen as **New** if their source evidence changes. Earlier notes and first-seen timestamps remain. Unchanged evidence preserves the current status. Legacy cases without an evidence fingerprint require review again on the first rescan if they already had a reviewed outcome.

Evidence downloads include customer contracts, usage, invoices and credits. Protect them like the source records. Fingerprints identify inputs; they do not establish accuracy or make the file store tamper-evident. Snapshot results record the status **at scan time**, not the latest review outcome. See [run-history behavior](docs/RECONCILIATION-RUNS.md), [API](docs/API.md) and [development guide](docs/DEVELOPMENT.md).

## What is implemented

- Fully functioning web dashboard: currency-separated portfolio KPIs, billing-period impact, leak categories, searchable and filterable investigations, source uploads, export, activity log, setup and demo reset.
- Go JSON API with one pinned PostgreSQL driver dependency, session login, signed HttpOnly cookies, CSRF enforcement, bounded upload sizes, security headers and rate limiting for sign-in.
- Four validated financial CSV sources: effective-dated rate cards, independently metered usage, finalized invoice lines and explicit service credits. A fifth optional customer-ID mapping CSV aligns external provider identifiers to canonical contract customers. Imports are keyed/upserted (retries do not duplicate the same ID).
- Deterministic reconciliation, supporting unit prices, included units, monthly minimums, paid/open invoice totals and explicit credits. It skips unfinished months (7-day grace), missing contracts, currency conflicts, draft/void invoices.
- Evidence in each finding: source IDs, billed/expected minor currency units, explanation and editable investigation status/notes. Human approval is mandatory for recovery actions.
- Optional server-side **read-only Stripe invoice synchronization** for explicitly mapped Price IDs. See below.
- Crash-resistant single-file persistence (temporary write + sync + atomic rename), Docker image/Compose, automated Go tests and architecture/roadmap/security docs.

## Supported data contract

Reconciliation works on one `(customer_id, YYYY-MM, metric)` tuple. Each monthly rate card must specify `customer_id`, `customer_name`, `metric`, `currency` (USD/EUR/GBP), `unit_price_cents`, `included_units`, `monthly_minimum_cents`, `effective_from`, and optional `effective_to` (YYYY-MM).

Usage records carry `usage_id`, `customer_id`, `period`, `metric`, `quantity`. Billing lines carry `line_id`, `invoice_id`, `customer_id`, `period`, `metric`, `quantity`, `amount_cents`, `currency`, `status` (`paid`, `open`, `draft`, or `void`). Credits carry `credit_id`, `customer_id`, `period`, `metric`, `amount_cents`, `reason`.

Money is always **integer minor currency units** (e.g. 100 cents = $1.00); usage quantity is integer. Supported currencies have 2 decimal places. Dates/periods are normalized to UTC. See [CSV samples](samples).

For an eligible period:

```text
chargeable_units = max(0, independent_usage - included_units)
expected_before_credit = max(chargeable_units * unit_price_cents, monthly_minimum_cents)
expected_net = max(0, expected_before_credit - approved_credits)
billed = sum(paid/open finalized invoice line amounts)
potential_variance = max(0, expected_net - billed)
```

Flags are **provisional**. A variance does not establish legal recoverability or a right to charge the customer. See [limitations](#limitations).

## Stripe setup (optional)

Set environment variables on the server only:

```bash
export STRIPE_RESTRICTED_KEY='your-Stripe-read-only-restricted-key'
export STRIPE_PRICE_MAP='{"price_your_usage_price_id":"api_calls"}'
```

Assign the key least-privilege read permissions to invoices and associated line items. The connector retrieves pages of invoices, fetches each invoice's line items, imports only paid/open invoices, explicitly mapped price IDs, supported currencies, and UTC full calendar-month lines. It subtracts reported line discount amounts and **does not create charges, invoices, refunds or payment intents**. Pagination is bounded; failure prevents partial persistence. Sync replaces only records previously imported by the Stripe source. Sync requires an explicit click.

Contract/usage records can use your own canonical customer IDs. If Stripe uses different `cus_...` IDs, import the optional [customer mapping CSV](samples/mappings.csv) first (`source=stripe`, external Stripe ID, canonical contract ID). Stripe records retain their original provider IDs in evidence; mappings are applied during reconciliation. Invoice line metadata and contract prices are not inferred. Don't mix manual CSV billing and Stripe for the same line items, or your totals may double count. Check invoice-level adjustments and applied credits separately; this connector is intentionally conservative and is **not an accounting-grade comprehensive Stripe reconciliation**.

Stripe API background: [Invoices](https://docs.stripe.com/api/invoices), [Invoice lines](https://docs.stripe.com/api/invoice-line-items), [Restricted keys](https://docs.stripe.com/keys#limit-access), [Meter summaries and eventual consistency](https://docs.stripe.com/api/billing/meter-event-summary/list).

## API

| Method | Route | Role |
| --- | --- | --- |
| GET | `/healthz` | Liveness |
| GET | `/api/me` | Session and environment capabilities |
| POST | `/api/login`, `/api/logout` | Admin session |
| GET | `/api/overview`, `/api/findings` | Portfolio and investigations |
| GET | `/api/datasets`, `/api/audit` | Imported source records and audit entries |
| GET | `/api/scans`, `/api/scans/{id}` | Saved run summaries and evidence snapshot |
| GET | `/api/scans/{id}/export` | Download saved evidence JSON |
| GET | `/api/findings/export` | CSV export (spreadsheet formula-safe) |
| POST | `/api/import/{contracts,usage,billing,credits}` | Multipart CSV; `file` field |
| POST | `/api/scan` | Reconcile eligible periods |
| POST | `/api/findings/{id}/status` | Change status and note |
| POST | `/api/stripe/sync` | Read-only Stripe invoice import |
| POST | `/api/demo/reset` | Reset dataset when `DEMO_MODE=true` |

All `/api/*` routes except `/api/me` and `/api/login` require a signed session, unless running a **localhost-only** passwordless demo. Mutations require `X-CSRF-Token` returned by `/api/me`; login uses JSON. No CORS is enabled.

## Test and build

```bash
go test ./...
go vet ./...
go build -o revenueleak ./cmd/server
```

The backend uses the Go standard library plus a pinned, MIT-licensed PostgreSQL driver; the UI uses browser-native JavaScript/CSS. The local demo does not require a PostgreSQL service, but Go modules must be downloaded on first build. The default filesystem datastore is sufficient for a single-workspace pilot, but production multi-tenant use requires a relational database, encrypted credential vault, RBAC, stronger job infrastructure and an independent security review. See [architecture](docs/ARCHITECTURE.md), [security](SECURITY.md), and [roadmap](docs/ROADMAP.md).

## Limitations

- This is a functional **pilot**, not a finished enterprise SaaS product or certified financial reporting tool.
- Single workspace; one local admin; JSON file persistence; no fine-grained role/tenant isolation or SSO.
- Contract math is limited to a single monthly metered metric, per-unit price, allowance, and monthly minimum. No graduated tiers, prepaid drawdown, FX conversion, amendment approvals, annual commits, or tax reconciliation.
- Complete independent usage and invoice imports are prerequisites. Missing usage, unmapped invoices, open/draft billing periods, credits, discount behaviors or unmodeled contracts may produce false positives or missed revenue.
- **Monetary findings** cover only customer/metric/month tuples present in a usage or paid/open billing record. A minimum commitment with *no* usage or finalized billing is flagged as a **source coverage check**, not a monetary variance. Data gaps cannot establish debt or collectibility.
- The Stripe connector only imports explicitly mapped monthly invoice lines (max 2000 invoices and 2000 lines per invoice per sync), not full billing-system state; there is no live meter-event ingestion or contract parser.
- The live findings list is recomputed from current inputs. Earlier findings remain in the latest 25 saved run snapshots; this is bounded history, not an indefinitely retained case ledger. Audit logs and snapshots are not tamper-evident.
- This software has not been independently penetration-tested, load-tested, or externally security-certified. Use synthetic data until your privacy, security and accounting requirements are reviewed.
