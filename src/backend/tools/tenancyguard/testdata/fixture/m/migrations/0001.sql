CREATE TABLE content.items (id uuid PRIMARY KEY, tenant_id uuid NOT NULL, deleted_at timestamptz);
ALTER TABLE content.items ENABLE ROW LEVEL SECURITY;
CREATE POLICY bad_read ON content.items FOR SELECT USING (tenant_id = infra.current_tenant_id() AND deleted_at IS NULL);
CREATE TABLE content.orphans (id uuid PRIMARY KEY, tenant_id uuid NOT NULL);
CREATE TABLE infra.outbox_like (id uuid, tenant_id uuid);
REVOKE DELETE, TRUNCATE ON ALL TABLES IN SCHEMA content FROM m_app;
DELETE FROM content.items WHERE id IS NULL;
