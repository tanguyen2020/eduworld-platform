// Workspace school_lms (C. School & Learning Management) — lazy chunk (F050-AC-02, F051-AC-10). Menus, paths and permissions come from the
// generated catalog (ADR-0090); this file only attaches a screen to each menu key. Later features replace
// `placeholderScreen` with their screen; a menu added to the catalog without a screen fails to compile (F051-AC-34).
// F052 (school administration): schools, classes, sis_sync, student_activity, access_control, settings, audit_logs.
import { placeholderScreen, WorkspaceRoutes, type MenuScreens } from '../../permissions/RequirePermission';
import { AuditScreen } from './audit/AuditScreen';
import { ClassesScreen } from './classes/ClassesScreen';
import { SchoolsScreen } from './schools/SchoolsScreen';
import { SettingsScreen } from './settings/SettingsScreen';
import { SisScreen } from './sis/SisScreen';
import { StudentActivityScreen } from './students/StudentActivityScreen';
import { AccessControlScreen } from './users/AccessControlScreen';

export const screens = {
  'menu.school_lms.overview': placeholderScreen,
  'menu.school_lms.schools': SchoolsScreen,
  'menu.school_lms.classes': ClassesScreen,
  'menu.school_lms.sis_sync': SisScreen,
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
  'menu.school_lms.student_activity': StudentActivityScreen,
  'menu.school_lms.access_control': AccessControlScreen,
  'menu.school_lms.installers': placeholderScreen,
  'menu.school_lms.settings': SettingsScreen,
  'menu.school_lms.trash': placeholderScreen,
  'menu.school_lms.audit_logs': AuditScreen,
} satisfies MenuScreens<'school_lms'>;

export default function SchoolLmsWorkspace() {
  return <WorkspaceRoutes workspace="school_lms" screens={screens} />;
}
