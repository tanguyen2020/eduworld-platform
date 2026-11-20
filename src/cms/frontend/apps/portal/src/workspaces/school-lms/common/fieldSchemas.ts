// Field limits of the F052 forms, copied from specs/openapi/eduworld-api.v1.yaml (ADR-0041, F052-AC-26). Each entry
// names the OpenAPI location it mirrors; `fieldSchemas.test.ts` fails when the spec and this file disagree, so the
// spec stays the single source (AGENTS R33). Used with `openapiRules` / `openapiInputProps` of ops-ui.
import type { OpenapiStringSchema } from '@eduworld/ops-ui';

export interface FieldSchema extends OpenapiStringSchema {
  /** `components/schemas/<Schema>/properties/<prop>` or `parameters/<Param>` in the merged spec. */
  source: string;
}

export const FIELDS = {
  schoolNameVi: { source: 'schemas/TenantLocalizedName/properties/vi', minLength: 1, maxLength: 255 },
  schoolNameEn: { source: 'schemas/TenantLocalizedName/properties/en', minLength: 1, maxLength: 255 },
  schoolAddress: { source: 'schemas/SchoolUpdateRequest/properties/address', maxLength: 500 },
  username: { source: 'schemas/TenantUserCreateRequest/properties/username', minLength: 3, maxLength: 128, pattern: '^[A-Za-z0-9._-]{3,128}$' },
  email: { source: 'schemas/TenantUserCreateRequest/properties/email', maxLength: 320 },
  emailUpdate: { source: 'schemas/TenantUserUpdateRequest/properties/email', maxLength: 320 },
  displayName: { source: 'schemas/TenantUserCreateRequest/properties/display_name', minLength: 1, maxLength: 128 },
  userSearch: { source: 'paths/listTenantUsers/q', minLength: 2, maxLength: 100 },
  classStudentSearch: { source: 'paths/listTenantClassStudents/q', minLength: 2, maxLength: 100 },
  roleCode: { source: 'schemas/RoleCode', maxLength: 64, pattern: '^[a-z][a-z0-9_]{1,63}$' },
  roleReason: { source: 'schemas/RoleInput/properties/reason', maxLength: 1000 },
  grantReason: { source: 'schemas/UserRoleGrantRequest/properties/reason', maxLength: 1000 },
  revokeReason: { source: 'parameters/ReasonQuery', minLength: 3, maxLength: 1000 },
  passwordResetReason: { source: 'schemas/StudentPasswordResetRequest/properties/reason', minLength: 3, maxLength: 500 },
  resolveReason: { source: 'schemas/SchoolReconciliationResolve/properties/reason', minLength: 3, maxLength: 500 },
  cleanupReason: { source: 'schemas/CleanupExclusionRequest/properties/reason', minLength: 3, maxLength: 500 },
  schoolSettingsReason: { source: 'schemas/SchoolSettings/properties/reason', minLength: 3, maxLength: 500 },
  auditAction: { source: 'paths/listTenantAuditLogs/action', maxLength: 128 },
  sisFileName: { source: 'schemas/SisImportCreate/properties/file_name', maxLength: 200, pattern: '\\.csv$' },
} as const satisfies Record<string, FieldSchema>;

/** Numeric limits mirrored from the spec (same check). */
export const LIMITS = {
  /** `SisImportCreate.size_bytes.maximum` — CONFIG_DEFAULTS `csv_import_max` (10 MB). */
  csvImportMaxBytes: { source: 'schemas/SisImportCreate/properties/size_bytes', maximum: 10485760 },
  /** `CleanupExclusionRequest.student_ids.maxItems`. */
  cleanupMaxStudents: { source: 'schemas/CleanupExclusionRequest/properties/student_ids', maxItems: 500 },
  /** `TenantSettings.inactive_student_cleanup_days` (0 = off). */
  cleanupDays: { source: 'schemas/TenantSettings/properties/inactive_student_cleanup_days', minimum: 0, maximum: 3650 },
  /** `TenantSettings.gameplay_chat_retention_days` (minimum 3, ADR-0033). */
  chatRetentionDays: { source: 'schemas/TenantSettings/properties/gameplay_chat_retention_days', minimum: 3, maximum: 365 },
  /** `SchoolSettings.daily_play_limit_minutes` (null = off, OQ-17). */
  dailyPlayLimit: { source: 'schemas/SchoolSettings/properties/daily_play_limit_minutes', minimum: 15, maximum: 300 },
} as const;
