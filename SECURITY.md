# Security, privacy and operational readiness

## Default posture

- Passwordless demo mode is **allowed only on localhost**; do not reverse proxy an unauthenticated demo.
- Private mode requires `ADMIN_PASSWORD` in the environment. Use a strong independent `SESSION_SECRET`, preserve only in a secrets manager, rotate on suspected compromise.
- Signed HttpOnly, SameSite=Strict cookies have a 24-hour expiry. Use HTTPS and a trusted TLS reverse proxy for any non-local access. Place the service behind a firewall, SSO or VPN until the production auth stack exists.
- APIs enforce a per-session CSRF token for changes; CSP limits scripts to same origin, and standard browser security headers are emitted.
- Sign-in is rate-limited per remote IP (memory-only). The reverse proxy must implement network-wide rate limits and DDoS controls.
- Stripe keys remain exclusively in server environment configuration. Use separate restricted, read-only API keys with minimum permissions, and avoid exposing the process environment in logs or diagnostic pages.
- No payments, invoices, customer emails, or refunds are created or modified by the application.

## Known pilot security gaps

1. No SSO, MFA, RBAC, multiple users, SCIM or audit attribution to individual users.
2. JSON persistence uses OS permissions but **no application-level at-rest encryption**. Encrypt the storage volume and backups; enforce OS access controls.
3. No multi-tenant isolation. Never co-host multiple real customers in one pilot workspace.
4. Audit events are mutable alongside the data store, not an externally secured append-only log.
5. Session cookies are marked `Secure` when the Go app directly terminates TLS. With TLS offload, ensure HTTPS enforcement at the proxy and cookie security validation before exposing externally.
6. CSP permits inline *styles* to support dynamic chart rendering; scripts remain same-origin only.
7. There is no formal privacy/consent workflow, data retention policy, automated backup/restore or tested disaster recovery.
8. No vulnerability scanner or independent penetration test has been performed.
9. Connector synchronization may be slow on large Stripe accounts; do not run concurrent full sync jobs in a production shared instance.
10. Billing correctness depends on complete and trusted source mappings. An underbilled finding is not a payable invoice.

## Production release checklist

- [ ] Threat model, security testing, dependency scanning, secrets scanning and independent review.
- [ ] Database migration and backup/restore drills.
- [ ] Tenant and permission model with integration tests for cross-tenant access.
- [ ] Modern OIDC/SSO and MFA, secret rotation, logout session revocation.
- [ ] Observability, synthetic probes and per-tenant ingest/reconciliation SLAs.
- [ ] Encrypted keys with KMS or secrets manager and fine-grained access scopes.
- [ ] Data-processing agreement, deletion/retention and legal review.
- [ ] Full pricing and invoice lifecycle support, plus finance signoff for each rule.
- [ ] Resilience and concurrent workload tests.
- [ ] Incident response, breach notification and change approval policies.

## Reporting vulnerabilities

Please do not test this pilot against third-party customer billing data or a public production system without permission. Report security findings through a private channel to the application owner. Do not include credentials or sensitive customer data in public issues.
