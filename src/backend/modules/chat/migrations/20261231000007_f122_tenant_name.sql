-- F122 T064 — tenant name in the identity projection of module chat (specs/122-portal-chat-safety: Clarifications
-- 2026-11-23; DATABASE_DESIGN §2.17; ADR-0041, ADR-0072). The Platform Safety Team sees the tenant of a case by name
-- (ModerationCase.tenant_name, listModerationCaseTenants) without a synchronous call to module identity: the consumer
-- chat.projection_identity copies `name` of tenant.created / tenant.updated (maxLength 255) into ref.tenants. Rows projected
-- before this migration keep NULL until the next tenant event (the API then shows the tenant code). Expand only: one
-- nullable column, nothing dropped or deleted (R12). ref.* is hand-written (not ent), so there is no entddl block.

ALTER TABLE ref.tenants ADD COLUMN name varchar(255);
