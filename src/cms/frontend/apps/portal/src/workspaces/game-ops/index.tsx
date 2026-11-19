// Workspace game_ops (A. Game Operations Console) — lazy chunk (F050-AC-02, F051-AC-10). Menus, paths and permissions come from the
// generated catalog (ADR-0090); this file only attaches a screen to each menu key. Later features replace
// `placeholderScreen` with their screen; a menu added to the catalog without a screen fails to compile (F051-AC-34).
import { placeholderScreen, WorkspaceRoutes, type MenuScreens } from '../../permissions/RequirePermission';

export const screens = {
  'menu.game_ops.dashboard': placeholderScreen,
  'menu.game_ops.world': placeholderScreen,
  'menu.game_ops.npcs': placeholderScreen,
  'menu.game_ops.quests': placeholderScreen,
  'menu.game_ops.minigames': placeholderScreen,
  'menu.game_ops.challenges': placeholderScreen,
  'menu.game_ops.bosses': placeholderScreen,
  'menu.game_ops.progression': placeholderScreen,
  'menu.game_ops.achievements': placeholderScreen,
  'menu.game_ops.items': placeholderScreen,
  'menu.game_ops.shop': placeholderScreen,
  'menu.game_ops.rewards': placeholderScreen,
  'menu.game_ops.events': placeholderScreen,
  'menu.game_ops.competition': placeholderScreen,
  'menu.game_ops.chat_assets': placeholderScreen,
  'menu.game_ops.media': placeholderScreen,
  'menu.game_ops.translations': placeholderScreen,
  'menu.game_ops.players': placeholderScreen,
  'menu.game_ops.parties': placeholderScreen,
  'menu.game_ops.review': placeholderScreen,
} satisfies MenuScreens<'game_ops'>;

export default function GameOpsWorkspace() {
  return <WorkspaceRoutes workspace="game_ops" screens={screens} />;
}
