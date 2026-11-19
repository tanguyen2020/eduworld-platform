// Workspace school_lms (C. School & Learning Management) — lazy chunk (F050-AC-02, F051-AC-10). Menus, paths and permissions come from the
// generated catalog (ADR-0090); this file only attaches a screen to each menu key. Later features replace
// `placeholderScreen` with their screen; a menu added to the catalog without a screen fails to compile (F051-AC-34).
import { placeholderScreen, WorkspaceRoutes, type MenuScreens } from '../../permissions/RequirePermission';

export const screens = {
  'menu.school_lms.overview': placeholderScreen,
  'menu.school_lms.schools': placeholderScreen,
  'menu.school_lms.classes': placeholderScreen,
  'menu.school_lms.sis_sync': placeholderScreen,
  'menu.school_lms.courses': placeholderScreen,
  'menu.school_lms.assignments': placeholderScreen,
  'menu.school_lms.game_activities': placeholderScreen,
  'menu.school_lms.schedules': placeholderScreen,
  'menu.school_lms.study_groups': placeholderScreen,
  'menu.school_lms.challenges': placeholderScreen,
  'menu.school_lms.clubs': placeholderScreen,
  'menu.school_lms.school_events': placeholderScreen,
  'menu.school_lms.school_dashboard': placeholderScreen,
  'menu.school_lms.results': placeholderScreen,
  'menu.school_lms.reports': placeholderScreen,
  'menu.school_lms.safety_notifications': placeholderScreen,
  'menu.school_lms.chat_safety': placeholderScreen,
  'menu.school_lms.student_activity': placeholderScreen,
  'menu.school_lms.access_control': placeholderScreen,
  'menu.school_lms.installers': placeholderScreen,
  'menu.school_lms.settings': placeholderScreen,
  'menu.school_lms.trash': placeholderScreen,
  'menu.school_lms.audit_logs': placeholderScreen,
} satisfies MenuScreens<'school_lms'>;

export default function SchoolLmsWorkspace() {
  return <WorkspaceRoutes workspace="school_lms" screens={screens} />;
}
