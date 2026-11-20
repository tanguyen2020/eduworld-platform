// Workspace content_studio (B. Learning Content Studio) — lazy chunk (F050-AC-02, F051-AC-10). Menus, paths and permissions come from the
// generated catalog (ADR-0090); this file only attaches a screen to each menu key. Later features replace
// `placeholderScreen` with their screen; a menu added to the catalog without a screen fails to compile (F051-AC-34).
import { placeholderScreen, WorkspaceRoutes, type MenuScreens } from '../../permissions/RequirePermission';
import { ActivitiesScreen } from './activities/ActivitiesScreen';
import { CurriculumTreePage } from './curriculum/CurriculumTreePage';
import { MediaLibraryPage } from './media/MediaLibraryPage';
import { QuestionBankScreen } from './questions/QuestionBankScreen';
import { RegradeScreen } from './regrade/RegradeScreen';
import { ReviewsScreen } from './review/ReviewQueuePage';
import { AssessmentsScreen, LearningPathsScreen, LessonsScreen, QuestionSetsScreen } from './screens';

export const screens = {
  'menu.content_studio.curriculum': CurriculumTreePage, // F053 US4
  'menu.content_studio.lessons': LessonsScreen, // F054 US4
  'menu.content_studio.activities': ActivitiesScreen, // F058
  'menu.content_studio.simulations': placeholderScreen,
  'menu.content_studio.question_bank': QuestionBankScreen, // F053 US1–US3, US5; F054 list, import, release
  'menu.content_studio.question_sets': QuestionSetsScreen, // F054 US4
  'menu.content_studio.assessments': AssessmentsScreen, // F054 US4
  'menu.content_studio.regrade': RegradeScreen, // F058 US4
  'menu.content_studio.media': MediaLibraryPage, // F054 US3
  'menu.content_studio.learning_paths': LearningPathsScreen, // F054 US1 (versions / releases)
  'menu.content_studio.reviews': ReviewsScreen, // F054 US2
  'menu.content_studio.translations': placeholderScreen,
} satisfies MenuScreens<'content_studio'>;

export default function ContentStudioWorkspace() {
  return <WorkspaceRoutes workspace="content_studio" screens={screens} />;
}
