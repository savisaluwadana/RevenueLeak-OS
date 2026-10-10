# Saved reconciliation runs

A saved run captures the exact records and findings at reconciliation time. Use the Saved runs workspace to inspect a result after the live data changes. Evidence contains provisional discrepancies; a saved calculation does not establish a right to bill or collect.

## What is captured

| Field | Meaning |
| --- | --- |
| id | Sequence-based workspace run ID |
| rule_version | Version of the monthly metered rule implementation |
| grace_days | Closed-period grace window used by the run |
| trigger | manual, import.contracts/usage/billing/credits, stripe.sync, or demo.seed |
| summary | UTC scan time, comparison/skip counts, findings and reopened cases |
| source_digest | SHA-256 of JSON-encoded source arrays in the recorded order |
| sources | Contracts, usage, billing and credits used in the run |
| findings | Results and review statuses as they existed at scan time |

The digest can be checked against those source arrays, but their accuracy still depends on the supplied records. It is not a signed attestation. Earlier snapshots do not change when a case is later approved or dismissed.

## Review invalidation

Each live case has a fingerprint of its effective contract and associated usage, finalized billing and credit records. Record order is normalized by stable source IDs. Changes to those inputs reopen an existing reviewed case as New, preserve its earlier notes and first-seen time, and show a review-required notice. Unchanged fingerprints retain the current status. An earlier datastore without fingerprints cannot prove that a reviewed outcome still applies; its reviewed cases reopen on the next scan.

A case that disappears from the live results is still inspectable in saved runs while those runs remain within retention. Reappearing cases are new live investigations; there is no separate permanent case-history table yet.

## Retention and compatibility

The latest 25 runs are retained newest first. Every successful manual scan, import or Stripe sync consumes a run slot. Export important runs before they expire. Runs and source updates persist atomically; failed transactions consume no slot. Old JSON datastores load with an empty history, and new history survives server restarts. Back up the data file before upgrading. No pre-upgrade snapshots are reconstructed.

Reset demo data deletes all live data and run history after the existing confirmation. It starts a new sequence, so do not interpret run IDs as globally unique. Downloaded evidence and backups remain outside app retention. This feature provides bounded, saved snapshots, not WORM storage, cryptographic audit integrity or a complete accounting ledger.
