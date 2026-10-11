# RevenueLeak OS — product strategy and build plan

## Product thesis

RevenueLeak OS is a **provider-neutral, read-only revenue assurance and prevention platform** for B2B SaaS and AI/API businesses. It compares contractual commitments, independent metered usage and finalized invoicing, and produces evidence-backed, human-reviewed discrepancies. It does **not** replace a billing platform or make autonomous collection decisions.

**Initial ICP:** B2B SaaS with (1) usage-based, minimum-commitment or hybrid commercial terms; (2) at least 20 commercial customers or meaningful contract complexity; (3) an independent product usage source; (4) a finance/billing owner who reviews invoices; and (5) enough at-risk revenue or review hours to justify a four-figure monthly subscription. Start with AI inference APIs, developer tooling, cloud infrastructure, data platforms and automation vendors. This is a hypothesis to validate in customer interviews.

**First sellable promise:** "Show me the monthly contract, usage and invoice mismatches with evidence, and let finance approve or dismiss them." Avoid promising any guaranteed recovery.

## What's built and what's still missing

| Capability | Status |
| --- | --- |
| CSV-based contract, usage, billing and credit reconciliation | Implemented for simple monthly metered contracts |
| Read-only Stripe invoices | Optional connector; not required |
| Reviewable monetary variance cases and saved evidence | Implemented for pilot |
| Missing independent usage **and** finalized billing for minimum-commitment months | Implemented as non-monetary coverage checks (12 closed months) |
| Secure independent customer organizations / tenant isolation | Not implemented |
| PostgreSQL, migrations, job queue, SSO, credential vault | Not implemented |
| Autonomous ingestion or webhook processing | Not implemented |
| Tiered pricing, annual commits, refunds, taxes, write-offs, proration | Not implemented |
| Contract OCR/AI interpretation and confirmation | Not implemented |
| Agent investigation, pre-invoice prevention and recovery actions | Not implemented |
| Paid subscriptions, checkout, metering and dunning for RevenueLeak | Not implemented |
| External security review and customer validation | Not completed |

## Source-independent connector architecture (target)

1. **Input adapters:** CSV initially; Stripe only for optional read-only invoice import. Next integrations should be prioritized from paid-pilot requests (e.g. Chargebee, QuickBooks, BigQuery, Snowflake), not by breadth of logos.
2. **Canonical source schema:** tenant, provider, external customer ID mapping, effective-dated contract, usage metric and period, invoice/line status, currency, explicit credits/adjustments, source fingerprint and ingestion watermark.
3. **Completeness checks:** missing source periods, stale jobs, duplicate events, billable metric mismatches, unrecognized customers and bad provider mappings; do not translate missing data into collectible cash.
4. **Deterministic pricing policies:** documented, versioned monetary rules with contract fixtures, reconciliation runs and discrepancy evidence.
5. **Approval workflows:** finance-confirmed status, owner, supporting evidence, corrections, and cash recovery tracked **separately** from potential leakage. AI may suggest investigation steps, never calculate unreviewed debt.
6. **Monitored automation:** durable jobs with retries, audit records and notifications. Before enabling outbound write operations, require explicit finance authorization and immutable approval records.

## Development priority and acceptance gates

### P0 — make it safe to onboard multiple paying companies

- [ ] PostgreSQL with migrations, backups and restoration test
- [ ] Organizations, users and memberships; enforce tenant scoping on **every** read/write including exports, coverage, scans and audit
- [ ] Login via OIDC (or suitably audited equivalent), secure session revocation, RBAC and auditable role changes
- [ ] KMS-encrypted connector secrets, rotation, scoped permissions and provider consent
- [ ] Durable ingest jobs, idempotent import cursors, signatures for webhooks and backfill/replay
- [ ] At least two real-world contract fixtures validated by the customer's finance team
- [ ] Observability, safe errors, privacy retention and deletion policies, baseline penetration review

**Gate:** No public multi-tenant rollout until tenant isolation and recovery/restore are independently tested.

### P1 — make it worth paying for

- [ ] Customer and provider ID mapping with provenance
- [ ] Usage completeness/late-arrival checks and source freshness indicators
- [ ] Contract effective dates, tiered prices, minimums, credits, and amendments supported by reviewed test fixtures
- [ ] Automated closed-period scans and exception notifications
- [ ] Case ownership, review notes, financial sign-off and correction workflow
- [ ] Verified recovered cash tracked separately from estimated underbilling and data gaps
- [ ] Evidence bundles and audit exports per verified finding

**Gate:** Three live paid pilots, low false-positive rates, and measurable finance team time saved or verified cash recovery, documented with approval.

### P2 — differential capabilities

- [ ] Contract extraction assistant that shows citations and requires term confirmation
- [ ] Pre-invoice simulation using draft invoice imports or provider APIs and reviewed rules
- [ ] AI investigation assistant that suggests root causes and draft tickets, never auto-charges
- [ ] Weekly trend detection, repeat offender root-cause groups and preventable exposure insights
- [ ] Additional connectors only when customers request and help justify subscription price

## Go-to-market: first $10k MRR experiment

**Illustrative revenue math:** 5 customers × $2,000 per month = $10,000 MRR. No sales, ARR or recovery rate is currently established. Avoid planning on speculative recovery-based commissions until financial and legal policies are clear.

**Suggested paid pilot:** 30 days, read-only, one business entity, one to two usage metrics, 3–6 completed billing months, structured CSV imports, 1 weekly findings review and written source assumptions. Charge a pilot fee to test willingness to pay; set pilot price and subscription terms based on conversations, not assumed market willingness.

**Discovery process:** 20 target-company interviews → 5 qualified data-access discussions → 3 signed pilots → validate precision, time saved and ROI → quote recurring monitoring. These numbers are a test funnel, not a forecast.

**Pricing hypotheses:**
- Starter $499/mo: monthly CSV checks, limited source periods and manual case review
- Growth $1,499/mo: automated runs, notifications and selected connectors
- Business $2,499/mo: advanced contracts, approvals and audit exports
- Enterprise $4,000+/mo (custom): SSO, support, advanced security and dedicated ingestion

Do not expose a paid plan until its advertised capabilities are implemented.

**Measure:** qualified customer conversations; paid pilots; time to first validated finding; source freshness; true/false positive rate by rule; confirmed dollars; collected dollars; reviewer time saved; churn risk; pilot-to-subscription conversion. Never combine missing source coverage minima with estimated recoverable revenue.

## Explicit non-goals

- Stripe-only analytics, payment retry optimization or another invoicing platform
- AI-generated amounts owed, autonomous collection letters or invoice edits
- Guaranteed financial recovery or "audited/certified" claims
- Broad ERP integration marketplace before evidence from real pilots

## Immediate work order

1. Merge the source coverage implementation after CI; use it to test missing-period detection without invented monetary totals.
2. Implement the P0 multi-tenant foundation in small reviewable changes: repository interface → PostgreSQL migrations → tenant identity and authorization → job queue.
3. Validate two customer contract models and add fixtures; then implement provider mapping and scheduled scans.
4. Only then expose customer-specific SaaS workspaces and start the first limited production pilots.

