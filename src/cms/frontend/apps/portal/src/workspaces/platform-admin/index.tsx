// Workspace platform_admin (D. Platform Administration) — lazy chunk (F050-AC-02, F051-AC-10). Menus, paths and permissions come from the
// generated catalog (ADR-0090); this file only attaches a screen to each menu key. Later features replace
// `placeholderScreen` with their screen; a menu added to the catalog without a screen fails to compile (F051-AC-34).
import { placeholderScreen, WorkspaceRoutes, type MenuScreens } from '../../permissions/RequirePermission';

export const screens = {
  'menu.platform_admin.tenants': placeholderScreen,
  'menu.platform_admin.access_control': placeholderScreen,
  'menu.platform_admin.tenant_data': placeholderScreen,
  'menu.platform_admin.catalog': placeholderScreen,
  'menu.platform_admin.school_policies': placeholderScreen,
  'menu.platform_admin.chat_safety': placeholderScreen,
  'menu.platform_admin.moderation_switches': placeholderScreen,
  'menu.platform_admin.break_glass': placeholderScreen,
  'menu.platform_admin.system_settings': placeholderScreen,
  'menu.platform_admin.feature_toggles': placeholderScreen,
  'menu.platform_admin.student_activity': placeholderScreen,
  'menu.platform_admin.reports': placeholderScreen,
  'menu.platform_admin.efficacy_studies': placeholderScreen,
  'menu.platform_admin.learning_experiments': placeholderScreen,
  'menu.platform_admin.data_requests': placeholderScreen,
  'menu.platform_admin.integrations': placeholderScreen,
  'menu.platform_admin.trash': placeholderScreen,
  'menu.platform_admin.audit_logs': placeholderScreen,
} satisfies MenuScreens<'platform_admin'>;

export default function PlatformAdminWorkspace() {
  return <WorkspaceRoutes workspace="platform_admin" screens={screens} />;
}
