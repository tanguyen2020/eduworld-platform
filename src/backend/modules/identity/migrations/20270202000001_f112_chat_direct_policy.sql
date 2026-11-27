-- Delta F112-7 (2026-11-28) — PlayerBootstrap.policies.chat.direct_message_enabled at the game login of module identity
-- (F013), without calling module chat on the login path (R30, ADR-0045). Hand-written tables accessed by explicit SQL.
-- Applied as identity_migrator.
--
-- Projections of the chat policy events of module chat (owner F120): chat_policy.updated (direct_message_enabled of
-- one level: platform, tenant, school) and chat_policy.common_tenant_social_updated (direct_enabled of a tenant
-- common). Written only by the consumer identity.chat_direct_policy; read by the game login. A projection of another
-- module's events: no soft-delete columns, one row per level updated in place (newer source_version only), never
-- removed by the application.

-- ---------- identity.chat_direct_policy_platform (G, projection): the platform level ----------
CREATE TABLE identity.chat_direct_policy_platform (
  scope_key              varchar(16) NOT NULL DEFAULT 'platform' CHECK (scope_key = 'platform'),
  direct_message_enabled boolean NOT NULL,
  source_version         bigint NOT NULL CHECK (source_version >= 1),
  updated_at             timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (scope_key)
);
REVOKE ALL ON identity.chat_direct_policy_platform FROM identity_app, identity_platform;
GRANT SELECT ON identity.chat_direct_policy_platform TO identity_app;
GRANT SELECT, INSERT, UPDATE ON identity.chat_direct_policy_platform TO identity_platform; -- consumer, platform event

-- ---------- identity.chat_direct_policies (T, projection): the common, tenant and school levels ----------
CREATE TABLE identity.chat_direct_policies (
  tenant_id              uuid NOT NULL REFERENCES platform.tenants (tenant_id),
  scope_type             varchar(16) NOT NULL CHECK (scope_type IN ('common', 'tenant', 'school')),
  scope_id               uuid NOT NULL, -- tenant_id for common/tenant, school_id for school
  direct_message_enabled boolean NOT NULL,
  source_version         bigint NOT NULL CHECK (source_version >= 1),
  updated_at             timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, scope_type, scope_id),
  CHECK (scope_type = 'school' OR scope_id = tenant_id)
);
ALTER TABLE identity.chat_direct_policies ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.chat_direct_policies FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_rw ON identity.chat_direct_policies FOR ALL USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE ALL ON identity.chat_direct_policies FROM identity_app, identity_platform;
GRANT SELECT, INSERT, UPDATE ON identity.chat_direct_policies TO identity_app, identity_platform;
