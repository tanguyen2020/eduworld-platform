// Workspace content_studio (B. Learning Content Studio) — lazy chunk (F050-AC-02, F051-AC-10). Menus, paths and permissions come from the
// generated catalog (ADR-0090); this file only attaches a screen to each menu key. Later features replace
// `placeholderScreen` with their screen; a menu added to the catalog without a screen fails to compile (F051-AC-34).
import { placeholderScreen, WorkspaceRoutes, type MenuScreens } from '../../permissions/RequirePermission';

export const screens = {
  'menu.content_studio.curriculum': placeholderScreen,
  'menu.content_studio.lessons': placeholderScreen,
  'menu.content_studio.activities': placeholderScreen,
  'menu.content_studio.simulations': placeholderScreen,
  'menu.content_studio.question_bank': placeholderScreen,
  'menu.content_studio.question_sets': placeholderScreen,
  'menu.content_studio.assessments': placeholderScreen,
  'menu.content_studio.regrade': placeholderScreen,
  'menu.content_studio.media': placeholderScreen,
  'menu.content_studio.learning_paths': placeholderScreen,
  'menu.content_studio.reviews': placeholderScreen,
  'menu.content_studio.translations': placeholderScreen,
} satisfies MenuScreens<'content_studio'>;

export default function ContentStudioWorkspace() {
  return <WorkspaceRoutes workspace="content_studio" screens={screens} />;
}
