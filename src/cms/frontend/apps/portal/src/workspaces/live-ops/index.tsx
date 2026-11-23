// Workspace live_ops (E. Game Live Operations) — lazy chunk (F050-AC-02, F051-AC-10). Menus, paths and permissions come from the
// generated catalog (ADR-0090); this file only attaches a screen to each menu key. Later features replace
// `placeholderScreen` with their screen; a menu added to the catalog without a screen fails to compile (F051-AC-34).
// F122 (Kiểm duyệt of the Platform Safety Team): moderation_cases (queue, case detail, school notifications), sanctions,
// tenant_terms (extra terms and categories of a target tenant).
import { placeholderScreen, WorkspaceRoutes, type MenuScreens } from '../../permissions/RequirePermission';
import { CasesScreen } from './moderation/CasesScreen';
import { SanctionsScreen } from './moderation/SanctionsScreen';
import { TenantTermsScreen } from './moderation/TenantTermsScreen';

export const screens = {
  'menu.live_ops.live_dashboard': placeholderScreen,
  'menu.live_ops.game_analytics': placeholderScreen,
  'menu.live_ops.moderation_cases': CasesScreen, // F122 US1, US2
  'menu.live_ops.sanctions': SanctionsScreen, // F122 US1
  'menu.live_ops.tenant_terms': TenantTermsScreen, // F122 US6
  'menu.live_ops.oncall': placeholderScreen,
  'menu.live_ops.incidents': placeholderScreen,
} satisfies MenuScreens<'live_ops'>;

export default function LiveOpsWorkspace() {
  return <WorkspaceRoutes workspace="live_ops" screens={screens} />;
}
