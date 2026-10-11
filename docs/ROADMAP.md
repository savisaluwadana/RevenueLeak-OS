# Product and development roadmap

## Positioning

**RevenueLeak OS** is revenue assurance for modern SaaS companies. It compares actual platform consumption, commercial terms, and invoice reality, and produces reviewable, source-backed discrepancy cases. It is deliberately **not** a billing platform, a collections agency, or a speculative LLM agent that decides what customers owe.

**Target buyer:** SaaS finance leaders, billing operations, founders, RevOps and monetization engineers. The initial ICP is a usage-based SaaS with at least one metered unit, signed rate cards and an independent authoritative usage table.

**Value hypothesis:** a finance team would pay for defensible findings that recover more than the annual cost, reduce manual reconciliation time, and avoid disputed charges. This hypothesis needs direct customer validation; pricing at $1,500–$4,000/month is an assumption, not demonstrated traction.

## Current release: v0.2 working pilot

- [x] Go backend, self-hosted UI, login, security headers and signed session.
- [x] Normalized contracts, usage, billing, credits with CSV ingestion and validation.
- [x] Deterministic monthly reconciliation, evidence, kinds, status and notes.
- [x] Dashboard, investigations, connector setup, audit log, CSV export and sample records.
- [x] Optional read-only Stripe finalized invoice pull with explicit price mapping.
- [x] Local persistence, Docker packaging, unit/API tests, technical docs.
- [x] Standalone repository, root-level CI, executable launcher and migration notes.
- [x] Saved run inputs/results, evidence JSON exports, 25-run retention and history UI.
- [x] Evidence changes reopen reviewed cases; regression tests cover restart, rollback and snapshot stability.
- [x] Non-monetary source coverage checks for missing minimum-commitment periods, bounded historical window, authenticated API, dashboard and saved evidence.
- [x] Optional single-process periodic reconciliation (`AUTO_SCAN_INTERVAL`), safe duplicate-scan suppression and cancellation tests.
- [x] Source-scoped CSV customer identity mapping for billing, usage and credits, with saved source provenance and regression tests.
- [x] PostgreSQL 16 tenant schema, migration checksum ledger, enforced RLS policies and cross-tenant SQL tests (schema only, not wired to the Go API).
- [x] Correct scheduled-scan fingerprint to include customer ID mapping changes.
- [x] Error-returning repository boundary for reads/writes, with server and scheduler failure-injection tests; PostgreSQL runtime adapter follows.
- [x] Experimental PostgreSQL JSONB workspace adapter with RLS, transactional updates, startup role checks and two-workspace integration tests.
- [ ] Production-normalized relational read/write paths, OIDC membership authorization, data migration and backup/restore drills.
- [ ] Mapping approvals, multi-tenant organization-level segregation, provider ID verification and collision reviews.
- [ ] Durable distributed scheduling, scheduled connector ingest, alerts and retries for production.
- [ ] Multi-tenant production security, payment-system certification and customer-validated ledger correctness.

## Phase 1: first paid pilot

1. Recruit 3–5 design partners using **any billing system or CSV**, with a clear independent authoritative product-usage export. Stripe remains an optional, read-only connector.
2. Choose one exact metric and contract interpretation per customer; sign written data-processing terms and define retention controls.
3. Run the engine **offline/read-only** against mutually confirmed closed billing periods; have customers label every finding (real, duplicate, credit, timing, disputed, cannot verify).
4. Track false-positive rate, verified recoverable amount, time to validate, and any cash collected only *after* finance confirms invoices and collections.
5. Add integration tests for each real contract model before enabling live ingest.

**Gate:** Do not claim a recovery rate or automate corrections until a finance reviewer has confirmed enough examples to measure precision accurately.

## Phase 2: production foundation

- Organizations/users, SSO/OIDC, RBAC and segregated tenancy.
- PostgreSQL-backed **private** workspace persistence is implemented as an opt-in JSONB adapter. Full normalized relational storage, shared-service tenant auth, backup/restore and production hardening are not implemented.
- Encrypted connector credentials with KMS, rotation and OAuth wherever supported.
- Asynchronous queues, retry/DLQ, idempotency checks, provider webhook signing and source-level checkpointing.
- Scale the pilot's saved run snapshots into relational, versioned reconciliation records with configurable retention, replay verification and tamper-evident approvals.
- Data minimization, PII policy, contractual consent and retention/deletion workflows.
- Monitoring (OTel), alerting, structured logs, audit export, rate limits and security audit.
- Customer-to-provider ID mapping and robust provider adjustments, credit notes, taxes and invoice-level discounts.

## Phase 3: differentiate through contract intelligence

- Contract ingestion assisted by document parsing, **human confirmation** of each extracted term.
- Tiered pricing, graduated pricing, prepaid commitments and drawdown, annual contract minima, rollover credits and mid-cycle changes.
- A flexible rules DSL with test fixtures and versioned rules.
- Evidence bundles with provenance, dual-control finance approval and reusable recovery playbooks.
- Usage providers: Snowflake, BigQuery, ClickHouse, Segment, S3 and event APIs.
- Billing systems: Chargebee, Recurly, Zuora and enterprise ERP connectors.
- Explainable forecast of future billing risk *separate from current recoverable money*.

## Phase 4: commercial expansion

- Compliance review matched to target markets (privacy/security, SOC 2 readiness as appropriate).
- Usage-based or risk-adjusted subscription packaging only after ROI is measured.
- Customer success playbooks, implementation SLAs, finance signoff and contractual recovery guardrails.
- Never silently post invoices, edit contract rates or collect funds on the basis of a model output.

## Important exclusions from current implementation

This release cannot accurately reconcile graduated tiering, minimums shared across metrics, annual prepayments, multiple currencies within a contract, write-offs, tax, refunds, disputed payments, or delayed/eventually consistent metering without a reviewed data model. Claims of "no bugs", enterprise readiness, or guaranteed recovered revenue would be premature.

## Next engineering milestone

1. Extend the error-returning repository and opt-in PostgreSQL JSONB adapter into normalized, versioned relational persistence; retain the local JSON demo adapter.
2. Add organizations, identities and memberships before exposing multiple customer workspaces; enforce tenant scoping for all coverage checks and exports.
3. Verify OIDC login, refresh/logout and per-role access across every import, run, finding and export route.
4. Move connector ingestion into durable jobs with checkpoints, concurrency limits and retries.
5. Add reviewed contract fixtures for tiering and source completeness before expanding monetary rules.

Current saved runs are a pilot foundation, not completion of the production foundation phase. Built-in SSO, **production-grade normalized** PostgreSQL persistence, end-to-end tenant authorization, live customer integration and production certification remain unimplemented.

## Commercial operating plan

See [PRODUCT-STRATEGY.md](PRODUCT-STRATEGY.md) for the $10k MRR validation hypothesis, initial customer profile, paid-pilot scope, tiered-pricing experiments and release gates. Product strategy and proposed prices are hypotheses, not verified sales or an implemented subscription checkout.
