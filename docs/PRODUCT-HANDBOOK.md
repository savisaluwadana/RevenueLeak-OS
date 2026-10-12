# RevenueLeak OS — complete product handbook

**Version:** repository pilot v0.2, with experimental private PostgreSQL persistence and a finance recovery assertion ledger.  
**Audience:** founders, SaaS finance, billing operations, product owners, implementers and security reviewers.  
**Truth in labeling:** **Built** means there is runnable code and tests; **experimental** means bounded pilot functionality; **planned** means the module is NOT in the product. Do not advertise planned features to paying customers as shipped.

## Product mission

RevenueLeak OS is a **read-only, evidence-first revenue assurance platform** for SaaS and API businesses. It compares commercial agreements, independent actual consumption, finalized billable invoice lines and explicit credits. It identifies **potential underbilling**, presents source evidence and supports operator review. It is neither an invoicing/collections system nor a certified accounting ledger.

The commercial differentiation we intend to build is: **detect and explain leakage before the invoice becomes final**, link every exception to contractual evidence, and guide a human finance reviewer through correction. This full vision is **not yet shipped**.

## Personas and intended jobs

| Persona | Decision / job | Present experience | Missing to serve properly |
| --- | --- | --- | --- |
| SaaS founder / owner | Estimate commercial revenue at risk | Currency-separated overview and investigations | Cohort trends, verified ROI, financial close signoff |
| Finance / billing analyst | Validate undercharging and exceptions | CSV import, evidence, review status, notes | Case assignment, maker/checker, recoveries verified from bank |
| Billing operations | Find invoice-vs-usage mismatches | Deterministic monthly pricing and optional Stripe read | More billing providers, adjustments, rate tiers |
| Revenue operations | Identify contract leakage | Effective-dated simple rate cards, source mapping | CRM/contract integrations, amendments, renewal cases |
| Platform / monetization engineer | Connect independent usage | Usage CSV, source IDs, fingerprints | Streaming ingestion, cursor tracking, dedupe across sources |
| Auditor / security lead | Explain why a result appeared | Saved runs (25), audits (500), source exports | Tamper-evident ledger, long-term retention and user attribution |
| Tenant administrator | Manage several customer workspaces | One private admin account per deployment only | OIDC/SAML, org membership, delegated RBAC and RLS API enforcement |

## Feature inventory

Status meanings: **Built** = implemented and covered by appropriate pilot checks, **Experimental** = implemented but unapproved for production, **Planned** = roadmap / design only.

| Product area | Detailed capability | Status |
| --- | --- | --- |
| Web | Responsive overview, filters, investigation drawer, exports, settings, data sources | Built |
| Login | Single admin secret, signed 24-hour session cookie, CSRF on mutations | Built (pilot) |
| Workspaces | Local JSON datastore, optional one-org PostgreSQL JSONB repository | Experimental |
| Source ingestion | Validated CSV for contracts, usage, billing, credits and mappings | Built |
| Billing | Optional read-only Stripe finalized invoice-line importer | Built, limited |
| Commercial rules | Effective-dated monthly per-metric unit price, included units and monthly minimum | Built |
| Matching | Provider-scoped explicit customer ID mappings; source IDs retained | Built |
| Reconciliation | Independent usage vs expected net price vs paid/open invoice lines | Built |
| Period correctness | Closed monthly windows, 7-day grace, skipped draft/void, missing contract / FX checks | Built |
| Source assurance | Missing-both usage and finalized invoice warning for minimum-commitment months, 12-month horizon | Built |
| Exception classes | Missing invoice, unbilled usage, price mismatch, minimum shortfall | Built, heuristic classification |
| Case workflow | New, reviewing, approved, false positive, resolved; notes and evidence invalidation | Built |
| Historical evidence | Immutable-in-app saved scan snapshot inputs, result and fingerprint; 25 recent runs | Built, bounded and not tamper-proof |
| Finance assertions | Distinct confirmed and operator-reported collection records, evidence digest, idempotency | Built, single-admin pilot; NO bank verification or dual approval |
| Activity trail | Last 500 workspace activity events | Built, mutable in source store |
| Auto checks | Optional scheduled scan over **currently stored** records | Built, single process |
| JSON / PostgreSQL migration | Explicit one-time import with dry-run and no-clobber, hash verification | Experimental |
| Backup / restore | Restricted manual PostgreSQL JSON export and import into **new** org | Experimental, no encryption / PITR |
| Multi-org login | OIDC, memberships, MFA, invitations, RBAC, SCIM | Planned |
| Production relational database | Normalized, tenant-scoped SQL repository and data migration | Planned; SQL schema exists but live data uses JSONB |
| Durable ingestion | Job queue, webhook signatures, checkpoints, retries, DLQ, notifications | Planned |
| Billing integrations | Chargebee, Recurly, Zuora, ERP/QuickBooks and broader CSV schemas | Planned; customer-demand gated |
| Usage integrations | Snowflake, BigQuery, ClickHouse, S3, warehouse & event API ingestion | Planned |
| Contract intelligence | Parse PDFs, extract citations, verify amendments, version commercial terms | Planned |
| Advanced pricing | Tiered/graduated rates, prepaid commitments, annual minima, credit rollover, proration, tax, refunds | Planned |
| Prevention | Pre-invoice dry-run, upcoming shortfalls, incomplete event warnings | Planned |
| AI agents | Evidence-linked root-cause research, recommendations, draft tasks and anomalies | Planned; never autonomously creates amounts owed |
| SaaS monetization | Self-service signup, subscriptions, plan gates, usage metering, dunning | Planned |
| Enterprise readiness | SSO/SAML, approval chains, configurable retention, observability, compliance review | Planned |

## What a user can do today

1. Start the demo on localhost, or configure a single-admin private workspace.
2. Import rate cards from `samples/contracts.csv` with effective month, customer ID, metric, minor-unit prices, included units and monthly minimum.
3. Optionally import source-to-canonical customer ID aliases. Review mappings manually.
4. Import independent usage, finalized invoice lines and explicit credits via validated CSV; alternatively configure a restricted read-only Stripe invoice integration.
5. Run a monthly reconciliation manually or enable `AUTO_SCAN_INTERVAL=24h` to check already-stored sources. A source import triggers recalculation automatically.
6. Review the dashboard by currency. Currency amounts are not summed together, and source coverage gaps are not added to potential leakage.
7. Open a finding, inspect the expected charge, billed lines, raw invoice IDs, usage IDs and explanation. Mark a finding approved, false positive, resolved or under review.
8. For an approved reviewed finding, record **operator-entered** finance-confirmed discrepancy and separately record reported cash collection within the confirmed current-evidence cap. Those entries do **not** charge customers or validate bank statements.
9. Inspect or export historical runs and evidence bundles. Export CSV findings and review the activity log.
10. For private PostgreSQL pilots, use guarded offline migration/backup commands; never use these to claim production disaster recovery.

## Clear semantics of the money

- **Potential discrepancy:** what the deterministic rule calculates from imported evidence; **not** an invoice, accounts receivable, or cash.
- **Confirmed for current evidence:** a human operator's explicitly recorded conclusion, with note and evidence version; **not** independently certified or second-person approved.
- **Collection recorded:** the operator states funds were collected; RevenueLeak does **not** look at a bank or cash ledger. Do not label it independently verified recovery.
- **Source coverage warning:** usage and finalized billing are both missing for an eligible contract minimum month; **not** collectible revenue.
- **Stale confirmation:** evidence changed/was removed; the old event remains in history, but its confirmation is excluded from current-evidence totals. Historical collection assertions remain labeled as operator-recorded.

**Never add amounts across currencies.** Historical collections and live potential discrepancies are separate sets and must not be added into a single 'money found' number.

## Supported pricing algorithm (v1 pilot only)

Grouping key: canonical customer, `YYYY-MM` billing period, usage metric. Choose the most recent rate card effective in that period. For eligible closed months after a seven-day grace period:

```text
billable_units = max(0, independently_metered_units - included_units)
base_charge_minor = billable_units * unit_price_minor
gross_expected_minor = max(base_charge_minor, contractual_monthly_minimum_minor)
net_expected_minor = max(0, gross_expected_minor - explicit_credit_minor)
billed_minor = sum(finalized paid/open invoice lines for same customer, period, metric and currency)
potential_discrepancy_minor = max(0, net_expected_minor - billed_minor)
```

Integers are minor currency units; supported current currencies are USD, EUR and GBP. Overflow/invalid currency and incomplete periods are skipped or flagged according to the current code; do not infer collectible balances when inputs are incomplete. Consult `docs/RECONCILIATION-RUNS.md` and `docs/API.md`.

## Business model hypothesis (NOT proven)

First target: usage-based B2B SaaS/API companies with independent product metering, varied commercial rate cards and finance-owned invoicing. Buyer: CFO, founder, billing manager or RevOps. Hypothesize that **five** customers at **$2,000/month** could reach **$10k MRR**, but this is illustrative, with **no validated market traction or production paid subscription capability**.

Suggested validation funnel: 20 conversations, 5 qualified data-access discussions, 3 signed read-only pilots, followed by reconciled true/false-positive review and a customer-specific paid offer. The software must show **financially correct, repeatable cases** and reduced analyst effort; do not sell an unreviewed 'guaranteed recovery' promise.

## Product scope, exclusions and release gates

**Appropriate now:** demo, synthetic samples, development, trusted single-client offline/read-only pilot with agreed consent and finance supervision.

**Not appropriate:** shared customer hosting, unapproved production data ingestion, automated invoices, collection letters, tax accounting, financial statements, AI-derived debt amounts, unverified compliance claims or enterprise-grade security commitments.

Read [architecture reference](ARCHITECTURE-REFERENCE.md), [database design](DATABASE-DESIGN.md), [end-to-end workflows](WORKFLOWS.md), [integration model](INTEGRATIONS.md), [security/operations](SECURITY-OPERATIONS.md), [test strategy](TEST-STRATEGY.md) and [implementation plan](IMPLEMENTATION-PLAN.md). The roadmap and P0 [GitHub issue #5](https://github.com/savisaluwadana/RevenueLeak-OS/issues/5) remain open until the release gates pass.
