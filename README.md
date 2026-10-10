# RevenueLeak OS

**A runnable, evidence-first SaaS revenue assurance pilot.** Reconcile independent usage, customer contract terms, approved credits, and finalized billing lines to identify *potential* underbilling. Designed for controlled finance investigation—not automatic debt collection.

![Status](https://img.shields.io/badge/release-0.2.0_pilot-green)

## Standalone repository

The application now lives at [savisaluwadana/RevenueLeak-OS](https://github.com/savisaluwadana/RevenueLeak-OS). See [migration notes](docs/MIGRATION.md), [architecture](docs/ARCHITECTURE.md), [roadmap](docs/ROADMAP.md), and [security](SECURITY.md).

## Quick start (no dependencies beyond Go 1.23)

```bash
DEMO_MODE=true go run ./cmd/server
```

Open **http://127.0.0.1:8080**. The default demo binds to localhost, seeds synthetic customers, and works without credentials or network access. Sample CSV files are available from the Data sources page. Data persists in `./data/revenueleak.json`; the **Settings → Reset demo data** button restores the sample dataset.

### Private workspace

```bash
export ADMIN_PASSWORD='replace-with-a-unique-long-password'
export SESSION_SECRET='replace-with-a-unique-random-signing-secret'
export ADDR='127.0.0.1:8080'
export DATA_FILE='./data/revenueleak.json'
go run ./cmd/server
```

Use a TLS reverse proxy for remote access, do not publicly expose the Go service, and keep server secrets out of Git. You can also run `docker compose up --build` after creating your own `.env` from `.env.example`. The Compose port is bound to localhost by default.

## Saved reconciliation runs (v0.2)

Every manual scan, CSV import, Stripe sync and demo seed now saves its inputs, calculated findings, rule version and SHA-256 input fingerprint in the same datastore transaction. Open **Saved runs → Inspect** to review an earlier result or download an evidence JSON bundle. The latest **25 runs** are retained; export important runs before they expire. A later source correction or review edit does not rewrite an earlier run. Scans that produce no findings are saved too.

Reviewed cases reopen as **New** if their source evidence changes. Earlier notes and first-seen timestamps remain. Unchanged evidence preserves the current status. Legacy cases without an evidence fingerprint require review again on the first rescan if they already had a reviewed outcome.

Evidence downloads include customer contracts, usage, invoices and credits. Protect them like the source records. Fingerprints identify inputs; they do not establish accuracy or make the file store tamper-evident. Snapshot results record the status **at scan time**, not the latest review outcome. See [run-history behavior](docs/RECONCILIATION-RUNS.md), [API](docs/API.md) and [development guide](docs/DEVELOPMENT.md).

## What is implemented

- Fully functioning web dashboard: currency-separated portfolio KPIs, billing-period impact, leak categories, searchable and filterable investigations, source uploads, export, activity log, setup and demo reset.
- Go standard-library JSON API, session login, signed HttpOnly cookies, CSRF enforcement, bounded upload sizes, security headers and rate limiting for sign-in.
- Four validated CSV sources: effective-dated rate cards, independently metered usage, finalized invoice lines and explicit service credits. Imports are keyed/upserted (retries do not duplicate the same ID).
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

Customers in your contract/usage import must use the *same* `cus_...` IDs returned by Stripe. Invoice line metadata and contract prices are not inferred. Don't mix manual CSV billing and Stripe for the same line items, or your totals may double count. Check invoice-level adjustments and applied credits separately; this connector is intentionally conservative and is **not an accounting-grade comprehensive Stripe reconciliation**.

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

The app uses **only the Go standard library** for the backend and browser-native JavaScript/CSS for the UI—no package installation required. The default filesystem datastore is sufficient for a single-workspace pilot, but production multi-tenant use requires a relational database, encrypted credential vault, RBAC, stronger job infrastructure and an independent security review. See [architecture](docs/ARCHITECTURE.md), [security](SECURITY.md), and [roadmap](docs/ROADMAP.md).

## Limitations

- This is a functional **pilot**, not a finished enterprise SaaS product or certified financial reporting tool.
- Single workspace; one local admin; JSON file persistence; no fine-grained role/tenant isolation or SSO.
- Contract math is limited to a single monthly metered metric, per-unit price, allowance, and monthly minimum. No graduated tiers, prepaid drawdown, FX conversion, amendment approvals, annual commits, or tax reconciliation.
- Complete independent usage and invoice imports are prerequisites. Missing usage, unmapped invoices, open/draft billing periods, credits, discount behaviors or unmodeled contracts may produce false positives or missed revenue.
- Scans cover only customer/metric/month tuples present in a usage or paid/open billing record. A minimum commitment with *no* usage or billing record for that month will not be flagged.
- The Stripe connector only imports explicitly mapped monthly invoice lines (max 2000 invoices and 2000 lines per invoice per sync), not full billing-system state; there is no live meter-event ingestion or contract parser.
- The live findings list is recomputed from current inputs. Earlier findings remain in the latest 25 saved run snapshots; this is bounded history, not an indefinitely retained case ledger. Audit logs and snapshots are not tamper-evident.
- This software has not been independently penetration-tested, load-tested, or externally security-certified. Use synthetic data until your privacy, security and accounting requirements are reviewed.
