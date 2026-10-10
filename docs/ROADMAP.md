# Product and development roadmap

## Positioning

**RevenueLeak OS** is revenue assurance for modern SaaS companies. It compares actual platform consumption, commercial terms, and invoice reality, and produces reviewable, source-backed discrepancy cases. It is deliberately **not** a billing platform, a collections agency, or a speculative LLM agent that decides what customers owe.

**Target buyer:** SaaS finance leaders, billing operations, founders, RevOps and monetization engineers. The initial ICP is a usage-based SaaS with at least one metered unit, signed rate cards and an independent authoritative usage table.

**Value hypothesis:** a finance team would pay for defensible findings that recover more than the annual cost, reduce manual reconciliation time, and avoid disputed charges. This hypothesis needs direct customer validation; pricing at $1,500–$4,000/month is an assumption, not demonstrated traction.

## Current release: v0.1 working pilot

- [x] Go backend, self-hosted UI, login, security headers and signed session.
- [x] Normalized contracts, usage, billing, credits with CSV ingestion and validation.
- [x] Deterministic monthly reconciliation, evidence, kinds, status and notes.
- [x] Dashboard, investigations, connector setup, audit log, CSV export and sample records.
- [x] Optional read-only Stripe finalized invoice pull with explicit price mapping.
- [x] Local persistence, Docker packaging, unit/API tests, technical docs.
- [ ] Multi-tenant production security, payment-system certification and customer-validated ledger correctness.

## Phase 1: first paid pilot

1. Recruit 3–5 design partners using Stripe + usage-based billing and a clear authoritative product-usage export.
2. Choose one exact metric and contract interpretation per customer; sign written data-processing terms and define retention controls.
3. Run the engine **offline/read-only** against mutually confirmed closed billing periods; have customers label every finding (real, duplicate, credit, timing, disputed, cannot verify).
4. Track false-positive rate, verified recoverable amount, time to validate, and any cash collected only *after* finance confirms invoices and collections.
5. Add integration tests for each real contract model before enabling live ingest.

**Gate:** Do not claim a recovery rate or automate corrections until a finance reviewer has confirmed enough examples to measure precision accurately.

## Phase 2: production foundation

- Organizations/users, SSO/OIDC, RBAC and segregated tenancy.
- PostgreSQL transactional persistence, migrations, backups and tested restore procedure.
- Encrypted connector credentials with KMS, rotation and OAuth wherever supported.
- Asynchronous queues, retry/DLQ, idempotency checks, provider webhook signing and source-level checkpointing.
- Immutable reconciliation-run snapshots, reproducible deterministic calculations and tamper-evident approvals.
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
