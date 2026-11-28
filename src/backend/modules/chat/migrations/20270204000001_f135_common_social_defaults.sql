-- Owner decision 2026-11-28 [Confirmed — owner 2026-11-28] (ADR-0078 §3 partly superseded, ADR-0104, CHILD_SAFETY §3.6):
-- in a tenant common, friends, direct messages and World chat are ON by default; System Admin keeps an incident switch
-- (putCommonTenantSocialOverride, reason + audit) that turns each off again. Area stays quick + emoji only.
-- Expand only (R12): one new column and two new column defaults; no row is changed or deleted (R21).
-- Stored rows keep their meaning: direct_enabled / world_enabled / area_free_text_enabled always held the effective state
-- of that tenant, so a row written before this migration stays exactly as it was (e.g. world_enabled = false stays off
-- until System Admin saves again). Only the meaning of an ABSENT row changes (code: policy.DefaultCommonOverride).
-- friends_enabled is new: friends of a common tenant used to follow settings_subset.common_social_enabled (never written,
-- so off); existing rows get the new owner default (true).

-- ---------- chat.common_tenant_social_overrides: friends switch + defaults aligned with common_tenant_social_defaults ----------
ALTER TABLE chat.common_tenant_social_overrides ADD COLUMN friends_enabled boolean NOT NULL DEFAULT true;
ALTER TABLE chat.common_tenant_social_overrides ALTER COLUMN direct_enabled SET DEFAULT true;
ALTER TABLE chat.common_tenant_social_overrides ALTER COLUMN world_enabled SET DEFAULT true;
