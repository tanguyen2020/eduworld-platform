// Input limits of workspace D (F057-AC-21, ADR-0041, AGENTS R28/R33): copied from specs/openapi/eduworld-api.v1.yaml —
// one entry per schema field, each with the pointer it comes from; `limits.test.ts` reads the spec and fails when a
// value drifts. Also the display threshold `inactive_student_cleanup_low_days_warning` of CONFIG_DEFAULTS (checked by
// the same test against docs/00-overview/CONFIG_DEFAULTS.md).

export interface FieldLimit {
  /** `components.schemas.<Schema>.properties.<field>` or `paths.<path>.<method>.parameters.<name>`. */
  source: string;
  minLength?: number;
  maxLength?: number;
  pattern?: string;
  minimum?: number;
  maximum?: number;
  maxItems?: number;
}

export const LIMITS = {
  tenantSearch: { source: 'paths./admin/tenants.get.parameters.q', maxLength: 100 },
  tenantCode: { source: 'components.schemas.TenantCreate.properties.code', minLength: 2, maxLength: 64 },
  tenantName: { source: 'components.schemas.TenantCreate.properties.name', minLength: 1, maxLength: 255 },
  tenantAlias: { source: 'components.schemas.TenantCreate.properties.keycloak_org_alias', maxLength: 63, pattern: '^[a-z0-9-]{3,63}$' },
  tenantCreateReason: { source: 'components.schemas.TenantCreate.properties.reason', maxLength: 500 },
  tenantUpdateReason: { source: 'components.schemas.TenantUpdate.properties.reason', minLength: 3, maxLength: 500 },
  assignTenantIds: { source: 'components.schemas.TenantAssignmentRequest.properties.tenant_ids', maxItems: 100 },
  unassignReason: { source: 'paths./admin/tenant-assignments.delete.requestBody.reason', maxLength: 500 },
  cloneResourceIds: { source: 'components.schemas.TenantCloneRequest.properties.resource_ids', maxItems: 500 },
  restoreReason: { source: 'components.schemas.RestoreRequest.properties.reason', minLength: 3, maxLength: 1000 },
  localizedName: { source: 'components.schemas.LocalizedName.properties.vi', minLength: 1, maxLength: 255 },
  countryReason: { source: 'components.schemas.CountryInput.properties.reason', maxLength: 1000 },
  provinceCode: { source: 'components.schemas.ProvinceInput.properties.code', maxLength: 64, pattern: '^[a-z]{2}(\\.old)?\\.[a-z0-9-]+$' },
  provinceOfficialCode: { source: 'components.schemas.ProvinceInput.properties.official_code', maxLength: 16 },
  provinceReason: { source: 'components.schemas.ProvinceInput.properties.reason', maxLength: 1000 },
  successionReason: { source: 'components.schemas.ProvinceSuccessionInput.properties.reason', maxLength: 1000 },
  schoolSearch: { source: 'paths./catalog/schools.get.parameters.q', minLength: 2, maxLength: 100 },
  cleanupDays: { source: 'components.schemas.StudentCleanupSettings.properties.default_days', minimum: 0, maximum: 3650 },
  cleanupNoticeDays: { source: 'components.schemas.StudentCleanupSettings.properties.notice_days', minimum: 0, maximum: 90 },
  cleanupThreshold: { source: 'components.schemas.StudentCleanupSettings.properties.approval_threshold_percent', minimum: 1, maximum: 100 },
  cleanupReason: { source: 'components.schemas.StudentCleanupSettings.properties.reason', minLength: 3, maxLength: 500 },
  chatRetentionDays: { source: 'components.schemas.ChatRetentionSettings.properties.gameplay_chat_retention_days', minimum: 3, maximum: 365 },
  chatEphemeralHours: { source: 'components.schemas.ChatRetentionSettings.properties.ephemeral_retention_hours', minimum: 1, maximum: 168 },
  chatReason: { source: 'components.schemas.ChatRetentionSettings.properties.reason', minLength: 3, maxLength: 500 },
  curfewTime: { source: 'components.schemas.PlatformCurfewSettings.properties.start', maxLength: 5, pattern: '^([01][0-9]|2[0-3]):[0-5][0-9]$' },
  curfewReason: { source: 'components.schemas.PlatformCurfewSettings.properties.reason', minLength: 3, maxLength: 500 },
  pageSize: { source: 'components.parameters.Limit', minimum: 1, maximum: 100 },
} as const satisfies Record<string, FieldLimit>;

/** CONFIG_DEFAULTS `inactive_student_cleanup_low_days_warning` (ADR-0032, ADMIN_PORTAL §3.10): display warning only. */
export const CLEANUP_LOW_DAYS_WARNING = 60;

/** Page size of the cursor lists of workspace D (≤ `Limit.maximum`). */
export const PAGE_SIZE = 50;

/** Poll period of asynchronous jobs (clone, school import) until a final status. UI cadence, not a business rule. */
export const JOB_POLL_INTERVAL_MS = 2000;

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
export const isUuid = (v: string): boolean => UUID.test(v);
