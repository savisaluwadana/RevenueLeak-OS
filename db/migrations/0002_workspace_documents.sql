-- Transitional single-workspace-per-organization persistence for the Go pilot.
-- The relational tenant schema remains the long-term normalized target.
-- Do not use this as an authorization or OIDC implementation.
CREATE TABLE IF NOT EXISTS revenueleak.workspace_documents (
  organization_id uuid PRIMARY KEY REFERENCES revenueleak.organizations(id),
  state jsonb NOT NULL DEFAULT '{}'::jsonb,
  version bigint NOT NULL DEFAULT 0 CHECK (version >= 0),
  updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE revenueleak.workspace_documents ENABLE ROW LEVEL SECURITY;
ALTER TABLE revenueleak.workspace_documents FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_scoped ON revenueleak.workspace_documents;
CREATE POLICY tenant_scoped ON revenueleak.workspace_documents
  USING (organization_id = revenueleak.current_organization())
  WITH CHECK (organization_id = revenueleak.current_organization());
