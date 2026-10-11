-- Execute with psql -X -v ON_ERROR_STOP=1 --single-transaction as an
-- isolated PostgreSQL superuser on a disposable CI database.
DO $$
DECLARE total_tables integer;
BEGIN
  SELECT COUNT(*) INTO total_tables
  FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
  WHERE n.nspname = 'revenueleak' AND c.relkind = 'r'
    AND c.relname IN (
      'organizations','memberships','customer_mappings','contracts',
      'usage_records','invoice_lines','credits','reconciliation_runs',
      'findings','approval_records','ingest_jobs','audit_events'
    )
    AND c.relrowsecurity AND c.relforcerowsecurity;
  IF total_tables <> 12 THEN
    RAISE EXCEPTION 'expected 12 forced-RLS tables, got %', total_tables;
  END IF;
END $$;

INSERT INTO revenueleak.organizations(id,slug,display_name) VALUES
('aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa','tenant-alpha','Tenant Alpha'),
('bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb','tenant-beta','Tenant Beta');

INSERT INTO revenueleak.contracts(organization_id, customer_id, customer_name, metric, currency, unit_price_minor, included_units, monthly_minimum_minor, effective_from)
VALUES
('aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa','shared-customer','Alpha Corp','api_calls','USD',10,0,1000,'2026-01-01'),
('bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb','shared-customer','Beta Corp','api_calls','USD',15,0,1500,'2026-01-01');

INSERT INTO revenueleak.invoice_lines(organization_id, source, external_line_id, external_invoice_id, external_customer_id, period, metric, currency, status, quantity, amount_minor)
VALUES
('aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa','stripe','same-external-line','in_alpha','shared-customer','2026-01-01','api_calls','USD','paid',50,500),
('bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb','stripe','same-external-line','in_beta','shared-customer','2026-01-01','api_calls','USD','paid',60,900);

INSERT INTO revenueleak.reconciliation_runs(id,organization_id,run_key,rule_version,source_digest) VALUES
('11111111-1111-4111-8111-111111111111','aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa','scan-1','rules-v1',repeat('a',64)),
('22222222-2222-4222-8222-222222222222','bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb','scan-1','rules-v1',repeat('b',64));

CREATE ROLE revenueleak_ci_runtime NOLOGIN;
GRANT USAGE ON SCHEMA revenueleak TO revenueleak_ci_runtime;
GRANT EXECUTE ON FUNCTION revenueleak.current_organization() TO revenueleak_ci_runtime;
GRANT SELECT ON revenueleak.organizations TO revenueleak_ci_runtime;
GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA revenueleak TO revenueleak_ci_runtime;
REVOKE UPDATE,DELETE ON revenueleak.audit_events FROM revenueleak_ci_runtime;

SET ROLE revenueleak_ci_runtime;
SET LOCAL app.organization_id = 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa';
DO $$
DECLARE visible_count integer;
DECLARE changed_count integer;
BEGIN
  SELECT COUNT(*) INTO visible_count FROM revenueleak.organizations;
  IF visible_count <> 1 THEN RAISE EXCEPTION 'organizations leaked across tenants'; END IF;
  SELECT COUNT(*) INTO visible_count FROM revenueleak.contracts;
  IF visible_count <> 1 THEN RAISE EXCEPTION 'contracts leaked across tenants'; END IF;
  SELECT COUNT(*) INTO visible_count FROM revenueleak.invoice_lines;
  IF visible_count <> 1 THEN RAISE EXCEPTION 'invoices leaked across tenants'; END IF;
  SELECT COUNT(*) INTO visible_count FROM revenueleak.reconciliation_runs;
  IF visible_count <> 1 THEN RAISE EXCEPTION 'run evidence leaked across tenants'; END IF;

  UPDATE revenueleak.invoice_lines SET amount_minor = 90000
    WHERE organization_id = 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb';
  GET DIAGNOSTICS changed_count = ROW_COUNT;
  IF changed_count <> 0 THEN RAISE EXCEPTION 'cross-tenant invoice update succeeded'; END IF;

  BEGIN
    INSERT INTO revenueleak.contracts(organization_id,customer_id,customer_name,metric,currency,unit_price_minor,included_units,monthly_minimum_minor,effective_from)
    VALUES ('bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb','injected','Injected','api_calls','USD',1,0,1,'2026-01-01');
    RAISE EXCEPTION 'unexpectedly inserted cross-tenant contract';
  EXCEPTION WHEN insufficient_privilege THEN NULL;
  END;

  BEGIN
    INSERT INTO revenueleak.findings(organization_id,reconciliation_run_id,case_key,customer_id,period,metric,currency,expected_minor,billed_minor,potential_minor)
    VALUES('aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa','22222222-2222-4222-8222-222222222222','bad-cross-ref','shared-customer','2026-01-01','api_calls','USD',100,0,100);
    RAISE EXCEPTION 'unexpectedly inserted cross-tenant evidence reference';
  EXCEPTION WHEN foreign_key_violation THEN NULL;
  END;
END $$;

SET LOCAL app.organization_id = '';
DO $$
DECLARE visible_count integer;
BEGIN
  SELECT COUNT(*) INTO visible_count FROM revenueleak.contracts;
  IF visible_count <> 0 THEN RAISE EXCEPTION 'default-deny failed for contracts'; END IF;
  SELECT COUNT(*) INTO visible_count FROM revenueleak.reconciliation_runs;
  IF visible_count <> 0 THEN RAISE EXCEPTION 'default-deny failed for evidence'; END IF;
END $$;
RESET ROLE;
-- A successful transaction rolls back at the end of the disposable CI database.
