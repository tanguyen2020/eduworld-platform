// Workspace platform_admin (D. Platform Administration) — lazy chunk (F050-AC-02, F051-AC-10, F057 T001/T044/T050).
// Menus, paths and permissions come from the generated catalog (ADR-0090); this file only attaches a screen to each
// menu key. F057 provides Tenant, Gán/Clone, Danh mục, Cấu hình hệ thống and Thùng rác; the other menus belong to
// F122/… and keep `placeholderScreen`. F059 attaches Người dùng & phân quyền, Nhật ký, Hoạt động đăng nhập game,
// Feature flag and Tích hợp. F035 attaches Nghiên cứu hiệu quả and Thử nghiệm A/B (research/). The route of the chunk is reached only when getMyPermissions lists the
// workspace (WorkspaceGate, F057-AC-18) — a display gate, the backend authorizes every call (AGENTS R20).
import { useTranslation } from 'react-i18next';
import { placeholderScreen, WorkspaceRoutes, type MenuScreens } from '../../permissions/RequirePermission';
import { AccessControlPage } from './access-control/AccessControlPage';
import { AuditLogsPage } from './audit/AuditLogsPage';
import { CatalogPage } from './catalog/CatalogPage';
import { FeatureFlagListPage } from './feature-flags/FeatureFlagListPage';
import { registerPlatformI18n } from './i18n';
import { IntegrationsPage } from './integrations/IntegrationsPage';
import { LoginEventsPage } from './login-events/LoginEventsPage';
import { PlatformReportsPage } from './reports/PlatformReportsPage';
import { EfficacyStudiesScreen } from './research/EfficacyStudiesScreen';
import { registerResearchI18n } from './research/i18n';
import { LearningExperimentsScreen } from './research/LearningExperimentsScreen';
import { SystemSettingsPage } from './settings/SystemSettingsPage';
import { TenantDataPage } from './tenantData/TenantDataPage';
import { TenantListPage } from './tenants/TenantListPage';
import { TrashPage } from './trash/TrashPage';

export const screens = {
  'menu.platform_admin.tenants': TenantListPage,
  'menu.platform_admin.access_control': AccessControlPage,
  'menu.platform_admin.tenant_data': TenantDataPage,
  'menu.platform_admin.catalog': CatalogPage,
  'menu.platform_admin.chat_safety': placeholderScreen,
  'menu.platform_admin.moderation_switches': placeholderScreen,
  'menu.platform_admin.system_settings': SystemSettingsPage,
  'menu.platform_admin.feature_toggles': FeatureFlagListPage,
  'menu.platform_admin.student_activity': LoginEventsPage,
  'menu.platform_admin.reports': PlatformReportsPage, // F056 US5
  'menu.platform_admin.efficacy_studies': EfficacyStudiesScreen,
  'menu.platform_admin.learning_experiments': LearningExperimentsScreen,
  'menu.platform_admin.data_requests': placeholderScreen,
  'menu.platform_admin.integrations': IntegrationsPage,
  'menu.platform_admin.trash': TrashPage,
  'menu.platform_admin.audit_logs': AuditLogsPage,
} satisfies MenuScreens<'platform_admin'>;

export default function PlatformAdminWorkspace() {
  const { i18n } = useTranslation();
  registerPlatformI18n(i18n);
  registerResearchI18n(i18n);
  return <WorkspaceRoutes workspace="platform_admin" screens={screens} />;
}
