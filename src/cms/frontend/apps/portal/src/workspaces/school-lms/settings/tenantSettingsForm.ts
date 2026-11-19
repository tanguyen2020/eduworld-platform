// Pure helpers of the tenant settings form (F052 T041, F052-AC-19, F052-AC-20).
import type { TenantSettings } from './useSettings';

/**
 * Warning threshold for `inactive_student_cleanup_days`: below it a cleanup may hit students away for the summer
 * holiday — ADMIN_PORTAL §3.10 "Cảnh báo khi đặt < 60 ngày" [Proposed]; owner document ADMIN_PORTAL.
 */
export const CLEANUP_WARNING_DAYS = 60;

export function cleanupWarning(days: number | null | undefined): boolean {
  return typeof days === 'number' && days > 0 && days < CLEANUP_WARNING_DAYS;
}

export type TenantSettingsForm = Pick<TenantSettings, 'allowed_hours' | 'inactive_student_cleanup_days' | 'gameplay_chat_retention_days' | 'default_locale'>;

const FIELDS = ['allowed_hours', 'inactive_student_cleanup_days', 'gameplay_chat_retention_days', 'default_locale'] as const;

/** JSON Merge Patch of the changed fields only (read-only `effective_cleanup_days` never sent). */
export function mergePatch(before: TenantSettings, after: TenantSettingsForm, editable: ReadonlySet<string>): Partial<TenantSettings> {
  const patch: Record<string, unknown> = {};
  for (const f of FIELDS) {
    if (!editable.has(f)) continue;
    const a = after[f] ?? null;
    const b = before[f] ?? null;
    if (JSON.stringify(a) !== JSON.stringify(b)) patch[f] = a;
  }
  return patch as Partial<TenantSettings>;
}
