# Documentation index and project map

This is the starting point for **everything RevenueLeak-OS currently does**, the architecture, data model, deployment, security, QA, and what must be built before commercial SaaS launch.

| Document | Audience | Contents |
| --- | --- | --- |
| [Product handbook](PRODUCT-HANDBOOK.md) | Founders, customers, product, finance | Mission, personas, use cases, full built/experimental/planned feature matrix, precise financial claims |
| [Architecture reference](ARCHITECTURE-REFERENCE.md) | CTOs, developers, architects | Current vs target architecture, component diagram, sequences, deployments, ADRs, trust boundaries |
| [Database design](DATABASE-DESIGN.md) | Backend, data, DBA | Current JSON/JSONB record dictionary, SQL schema and indexes, ER diagrams, future normalized tables, migration invariants |
| [Workflows](WORKFLOWS.md) | Finance, product, engineers | Onboarding, mapping, ingestion, deterministic reconciliation, investigations, recovery, backup, agent workflows |
| [Integrations and agents](INTEGRATIONS.md) | Integrations, platform, AI engineers | Current CSV/Stripe inputs, planned connector contracts, ingestion jobs, agent orchestration and safety |
| [Security and operations](SECURITY-OPERATIONS.md) | Security, SRE, operations | Threat model, deploy, privacy, access control gaps, runbooks, observability, release gates |
| [Test strategy](TEST-STRATEGY.md) | QA, backend, finance | Existing CI, golden financial fixtures, tenancy/security negative tests, recovery checks, definition of done |
| [Implementation plan](IMPLEMENTATION-PLAN.md) | Product, engineering leads | M0-M7 staged P0/P1/P2 work, PR slices, acceptance criteria, business validation |
| [API reference](API.md) | Developers | Existing routes, payloads, auth/CSRF, scan evidence and recovery assertions |
| [PostgreSQL foundation](POSTGRESQL-FOUNDATION.md) | DBA, SRE, implementers | Migration and privilege setup, private runtime, guarded JSON import, backup and test restore |
| [Reconciliation run behavior](RECONCILIATION-RUNS.md) | Finance, engineers | 25-run retention, fingerprints, historical evidence and review invalidation |
| [Local development](DEVELOPMENT.md) | Contributors | Start commands, tests, browser smoke and CI |
| [Product strategy](PRODUCT-STRATEGY.md) | Founder, GTM | ICP, positioning, pricing hypotheses, paid-pilot validation funnel |
| [Roadmap](ROADMAP.md) | Everyone | Checklist for built pilot versus blocked production capabilities |
| [Migration notes](MIGRATION.md) | Maintainers | Standalone repository origins and previous migration history |
| [Security notice](../SECURITY.md) | Everyone | Pilot security limits and vulnerability handling |
| [Main README](../README.md) | Everyone | Quick start, supported features and limitations |

## Fast paths

**I want to try the product:** [README](../README.md) → [Local development](DEVELOPMENT.md) → [Product handbook](PRODUCT-HANDBOOK.md) → [Workflows](WORKFLOWS.md).

**I want to add a feature:** [Architecture](ARCHITECTURE-REFERENCE.md) → [Database design](DATABASE-DESIGN.md) → [API](API.md) → [Tests](TEST-STRATEGY.md) → [Implementation plan](IMPLEMENTATION-PLAN.md).

**I want to sell a pilot:** [Product handbook](PRODUCT-HANDBOOK.md) → [Product strategy](PRODUCT-STRATEGY.md) → [Security and operations](SECURITY-OPERATIONS.md) → [Workflows](WORKFLOWS.md). **Do not represent planned features as already delivered.**

**I want to deploy for one private pilot:** [PostgreSQL foundation](POSTGRESQL-FOUNDATION.md) → [Security and operations](SECURITY-OPERATIONS.md) → [Test strategy](TEST-STRATEGY.md). No public multi-tenant deployment is currently safe.

**I want to build agentic AI:** [Integrations and agents](INTEGRATIONS.md) → [Architecture target](ARCHITECTURE-REFERENCE.md) → [Implementation milestones M4-M6](IMPLEMENTATION-PLAN.md). Keep deterministic financial arithmetic and human-approved actions separate from agents.

## Naming and status

- **Built:** runnable code exists; does not mean customer-grade accuracy.
- **Experimental:** implemented with explicit private-pilot limits.
- **Planned / TARGET:** specification and acceptance criteria only, not available to users.
- **Financially verified:** only when the customer's responsible finance reviewer verifies the result from independent source-of-truth records.
- **Bank/provider-verified collected cash:** not a supported claim in the current application.

The release gate is [P0 Issue #5](https://github.com/savisaluwadana/RevenueLeak-OS/issues/5). All materials should be reviewed when the code evolves.
