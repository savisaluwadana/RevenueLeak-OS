-- RevenueLeak OS PostgreSQL foundation, version 0001.
-- Designed for PostgreSQL 15+. This schema is NOT yet wired to the pilot API.
-- Apply with a dedicated migration owner. The runtime DB role must NOT own
-- tables, have BYPASSRLS, be superuser, or have CREATE on this schema.
CREATE SCHEMA IF NOT EXISTS revenueleak;
REVOKE ALL ON SCHEMA revenueleak FROM PUBLIC;

CREATE OR REPLACE FUNCTION revenueleak.current_organization()
RETURNS uuid
LANGUAGE sql STABLE
AS $$
  SELECT NULLIF(current_setting('app.organization_id', true), '')::uuid;
$$;

CREATE TABLE IF NOT EXISTS revenueleak.organizations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slug text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9][a-z0-9-]{1,62}$'),
  display_name text NOT NULL CHECK (length(trim(display_name)) > 0),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS revenueleak.memberships (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES revenueleak.organizations(id),
  identity_issuer text NOT NULL,
  identity_subject text NOT NULL,
  role text NOT NULL CHECK (role IN ('owner','admin','finance_approver','analyst','viewer')),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (organization_id, id),
  UNIQUE (organization_id, identity_issuer, identity_subject),
  CHECK (length(trim(identity_issuer)) > 0 AND length(trim(identity_subject)) > 0)
);

CREATE TABLE IF NOT EXISTS revenueleak.customer_mappings (
  organization_id uuid NOT NULL REFERENCES revenueleak.organizations(id),
  source text NOT NULL CHECK (source IN ('stripe','billing_csv','usage_csv','credits_csv')),
  external_customer_id text NOT NULL,
  canonical_customer_id text NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (organization_id, source, external_customer_id),
  CHECK (length(trim(external_customer_id)) > 0 AND length(trim(canonical_customer_id)) > 0)
);

CREATE TABLE IF NOT EXISTS revenueleak.contracts (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES revenueleak.organizations(id),
  customer_id text NOT NULL,
  customer_name text NOT NULL,
  metric text NOT NULL,
  currency char(3) NOT NULL CHECK (currency IN ('USD','EUR','GBP')),
  unit_price_minor bigint NOT NULL CHECK (unit_price_minor >= 0),
  included_units bigint NOT NULL CHECK (included_units >= 0),
  monthly_minimum_minor bigint NOT NULL CHECK (monthly_minimum_minor >= 0),
  effective_from date NOT NULL,
  effective_to date,
  recorded_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (organization_id, id),
  UNIQUE (organization_id, customer_id, metric, effective_from),
  CHECK (date_trunc('month', effective_from)::date = effective_from),
  CHECK (effective_to IS NULL OR (date_trunc('month', effective_to)::date = effective_to AND effective_to >= effective_from))
);
CREATE INDEX IF NOT EXISTS contracts_org_customer_metric_idx ON revenueleak.contracts (organization_id, customer_id, metric);

CREATE TABLE IF NOT EXISTS revenueleak.usage_records (
  organization_id uuid NOT NULL REFERENCES revenueleak.organizations(id),
  source text NOT NULL,
  external_usage_id text NOT NULL,
  external_customer_id text NOT NULL,
  period date NOT NULL,
  metric text NOT NULL,
  quantity bigint NOT NULL CHECK (quantity >= 0),
  recorded_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (organization_id, source, external_usage_id),
  CHECK (date_trunc('month', period)::date = period)
);
CREATE INDEX IF NOT EXISTS usage_org_period_idx ON revenueleak.usage_records (organization_id, period, external_customer_id);

CREATE TABLE IF NOT EXISTS revenueleak.invoice_lines (
  organization_id uuid NOT NULL REFERENCES revenueleak.organizations(id),
  source text NOT NULL,
  external_line_id text NOT NULL,
  external_invoice_id text NOT NULL,
  external_customer_id text NOT NULL,
  period date NOT NULL,
  metric text NOT NULL,
  currency char(3) NOT NULL CHECK (currency IN ('USD','EUR','GBP')),
  status text NOT NULL CHECK (status IN ('paid','open','draft','void')),
  quantity bigint NOT NULL CHECK (quantity >= 0),
  amount_minor bigint NOT NULL CHECK (amount_minor >= 0),
  recorded_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (organization_id, source, external_line_id),
  CHECK (date_trunc('month', period)::date = period)
);
CREATE INDEX IF NOT EXISTS invoice_lines_org_period_idx ON revenueleak.invoice_lines (organization_id, period, external_customer_id);

CREATE TABLE IF NOT EXISTS revenueleak.credits (
  organization_id uuid NOT NULL REFERENCES revenueleak.organizations(id),
  source text NOT NULL,
  external_credit_id text NOT NULL,
  external_customer_id text NOT NULL,
  period date NOT NULL,
  metric text NOT NULL,
  amount_minor bigint NOT NULL CHECK (amount_minor >= 0),
  reason text NOT NULL,
  recorded_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (organization_id, source, external_credit_id),
  CHECK (date_trunc('month', period)::date = period)
);

CREATE TABLE IF NOT EXISTS revenueleak.reconciliation_runs (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES revenueleak.organizations(id),
  run_key text NOT NULL,
  rule_version text NOT NULL,
  source_digest char(64) NOT NULL,
  summary jsonb NOT NULL DEFAULT '{}'::jsonb,
  source_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (organization_id, id),
  UNIQUE (organization_id, run_key)
);
CREATE INDEX IF NOT EXISTS reconciliation_runs_org_time_idx ON revenueleak.reconciliation_runs (organization_id, created_at DESC);

CREATE TABLE IF NOT EXISTS revenueleak.findings (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES revenueleak.organizations(id),
  reconciliation_run_id uuid NOT NULL,
  case_key text NOT NULL,
  customer_id text NOT NULL,
  period date NOT NULL,
  metric text NOT NULL,
  currency char(3) NOT NULL CHECK (currency IN ('USD','EUR','GBP')),
  expected_minor bigint NOT NULL CHECK (expected_minor >= 0),
  billed_minor bigint NOT NULL CHECK (billed_minor >= 0),
  potential_minor bigint NOT NULL CHECK (potential_minor >= 0),
  status text NOT NULL DEFAULT 'new' CHECK (status IN ('new','reviewing','approved','false_positive','resolved')),
  reviewer_note text NOT NULL DEFAULT '',
  evidence jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (organization_id, id),
  FOREIGN KEY (organization_id, reconciliation_run_id)
    REFERENCES revenueleak.reconciliation_runs (organization_id, id),
  CHECK (date_trunc('month', period)::date = period)
);
CREATE INDEX IF NOT EXISTS findings_org_status_idx ON revenueleak.findings (organization_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS revenueleak.approval_records (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES revenueleak.organizations(id),
  finding_id uuid NOT NULL,
  actor_membership_id uuid NOT NULL,
  action text NOT NULL CHECK (action IN ('approved','rejected','returned')),
  reason text NOT NULL DEFAULT '',
  recorded_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (organization_id, id),
  FOREIGN KEY (organization_id, finding_id) REFERENCES revenueleak.findings (organization_id, id),
  FOREIGN KEY (organization_id, actor_membership_id) REFERENCES revenueleak.memberships (organization_id, id)
);

CREATE TABLE IF NOT EXISTS revenueleak.ingest_jobs (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES revenueleak.organizations(id),
  connector text NOT NULL,
  idempotency_key text NOT NULL,
  status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','running','succeeded','failed','dead_letter')),
  attempt_count integer NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
  payload_digest char(64),
  next_attempt_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (organization_id, id),
  UNIQUE (organization_id, connector, idempotency_key)
);
CREATE INDEX IF NOT EXISTS ingest_jobs_org_queue_idx ON revenueleak.ingest_jobs (organization_id, status, next_attempt_at);

CREATE TABLE IF NOT EXISTS revenueleak.audit_events (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES revenueleak.organizations(id),
  actor_membership_id uuid,
  action text NOT NULL,
  details jsonb NOT NULL DEFAULT '{}'::jsonb,
  recorded_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (organization_id, id),
  FOREIGN KEY (organization_id, actor_membership_id) REFERENCES revenueleak.memberships (organization_id, id)
);
CREATE INDEX IF NOT EXISTS audit_events_org_time_idx ON revenueleak.audit_events (organization_id, recorded_at DESC);

-- Defense in depth: every tenant-owned table is protected, even against its
-- table owner (superuser/BYPASSRLS still bypass, so never use that for the app).
ALTER TABLE revenueleak.organizations ENABLE ROW LEVEL SECURITY;
ALTER TABLE revenueleak.organizations FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS organization_scoped ON revenueleak.organizations;
CREATE POLICY organization_scoped ON revenueleak.organizations
  USING (id = revenueleak.current_organization())
  WITH CHECK (id = revenueleak.current_organization());

DO $$
DECLARE tenant_table text;
BEGIN
  FOREACH tenant_table IN ARRAY ARRAY[
    'memberships','customer_mappings','contracts','usage_records',
    'invoice_lines','credits','reconciliation_runs','findings',
    'approval_records','ingest_jobs','audit_events'
  ] LOOP
    EXECUTE format('ALTER TABLE revenueleak.%I ENABLE ROW LEVEL SECURITY', tenant_table);
    EXECUTE format('ALTER TABLE revenueleak.%I FORCE ROW LEVEL SECURITY', tenant_table);
    EXECUTE format('DROP POLICY IF EXISTS tenant_scoped ON revenueleak.%I', tenant_table);
    EXECUTE format(
      'CREATE POLICY tenant_scoped ON revenueleak.%I USING (organization_id = revenueleak.current_organization()) WITH CHECK (organization_id = revenueleak.current_organization())',
      tenant_table
    );
  END LOOP;
END $$;

-- Grants must be applied to a separate least-privilege runtime role after
-- migration. Do NOT connect the app using the migration/schema owner role.
