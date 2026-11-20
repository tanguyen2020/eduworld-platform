// Workspace school_lms (C. School & Learning Management) — lazy chunk (F050-AC-02, F051-AC-10). Menus, paths and permissions come from the
// generated catalog (ADR-0090); this file only attaches a screen to each menu key. Later features replace
// `placeholderScreen` with their screen; a menu added to the catalog without a screen fails to compile (F051-AC-34).
// F052 (school administration): schools, classes, sis_sync, student_activity, access_control, settings, audit_logs;
// F054: trash (Thùng rác of the tenant); F055 (LMS): courses, assignments, game_activities, schedules, study_groups;
// F056 (School Learning Dashboard and reports): school_dashboard, results, reports.
import { placeholderScreen, WorkspaceRoutes, type MenuScreens } from '../../permissions/RequirePermission';
import { AuditScreen } from './audit/AuditScreen';
import { ClassesScreen } from './classes/ClassesScreen';
import { ReportsScreen } from './dashboard/reports/ReportsScreen';
import { ResultsScreen } from './dashboard/results/ResultsScreen';
import { SchoolDashboardScreen } from './dashboard/SchoolDashboardScreen';
import { AssignmentsScreen } from './lms/assignments/AssignmentsScreen';
import { CoursesScreen } from './lms/courses/CoursesScreen';
import { GameActivitiesScreen } from './lms/game-activities/GameActivitiesScreen';
import { SchedulesScreen } from './lms/schedule/SchedulesScreen';
import { StudyGroupsScreen } from './lms/study-groups/StudyGroupsScreen';
import { SchoolsScreen } from './schools/SchoolsScreen';
import { SettingsScreen } from './settings/SettingsScreen';
import { SisScreen } from './sis/SisScreen';
import { StudentActivityScreen } from './students/StudentActivityScreen';
import { TenantTrashPage } from './trash/TenantTrashPage';
import { AccessControlScreen } from './users/AccessControlScreen';

export const screens = {
  'menu.school_lms.overview': placeholderScreen,
  'menu.school_lms.schools': SchoolsScreen,
  'menu.school_lms.classes': ClassesScreen,
  'menu.school_lms.sis_sync': SisScreen,
  'menu.school_lms.courses': CoursesScreen, // F055 US3
  'menu.school_lms.assignments': AssignmentsScreen, // F055 US1, US2, US5
  'menu.school_lms.game_activities': GameActivitiesScreen, // F055-AC-22
  'menu.school_lms.schedules': SchedulesScreen, // F055 US4
  'menu.school_lms.study_groups': StudyGroupsScreen, // F055 US4
  'menu.school_lms.challenges': placeholderScreen,
  'menu.school_lms.clubs': placeholderScreen,
  'menu.school_lms.school_events': placeholderScreen,
  'menu.school_lms.school_dashboard': SchoolDashboardScreen, // F056 US1, US2, US6
  'menu.school_lms.results': ResultsScreen, // F056 US3
  'menu.school_lms.reports': ReportsScreen, // F056 US4, US5
  'menu.school_lms.safety_notifications': placeholderScreen,
  'menu.school_lms.chat_safety': placeholderScreen,
  'menu.school_lms.student_activity': StudentActivityScreen,
  'menu.school_lms.access_control': AccessControlScreen,
  'menu.school_lms.installers': placeholderScreen,
  'menu.school_lms.settings': SettingsScreen,
  'menu.school_lms.trash': TenantTrashPage, // F054 US5
  'menu.school_lms.audit_logs': AuditScreen,
} satisfies MenuScreens<'school_lms'>;

export default function SchoolLmsWorkspace() {
  return <WorkspaceRoutes workspace="school_lms" screens={screens} />;
}
