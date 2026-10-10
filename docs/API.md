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
| GET | /api/scans | `{runs: [...], retention_limit: 25}`; summaries only |
| GET | /api/scans/{id} | `{run: {...}}`; recorded sources and results |
| GET | /api/scans/{id}/export | Download `{schema_version: "1", run: {...}}` as JSON |

`kind` is contracts, usage, billing or credits. Source CSV templates are in `samples/`. A finding status payload is `{"status":"reviewing","note":"..."}`; allowed statuses are new, reviewing, approved, false_positive and resolved.

Saved-run routes require a private session and return `401` without it. Missing or expired runs return `404`; JSON errors have an `error` string. Run JSON responses use `Cache-Control: no-store`. Downloads include customer contracts and billing inputs. The source digest identifies the saved inputs, not financial collectibility or external certification.
