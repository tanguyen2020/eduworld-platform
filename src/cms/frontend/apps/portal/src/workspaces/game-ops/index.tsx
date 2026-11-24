// Workspace game_ops (A. Game Operations Console) — lazy chunk (F050-AC-02, F051-AC-10, F079-AC-18). Menus, paths and
// permissions come from the generated catalog (ADR-0090); this file only attaches a screen to each menu key. The chunk
// is never a security layer: module game authorizes every request (AGENTS R20). Menus of other features keep the
// placeholder: competition (F108), chat assets and translations (F082), players and parties (F080).
import { placeholderScreen, WorkspaceRoutes, type MenuScreens } from '../../permissions/RequirePermission';
import { configMenuScreen } from './ConfigMenuScreen';
import { GameDashboard } from './dashboard/GameDashboard';
import { GameMediaPage } from './media/GameMediaPage';
import { ReviewQueuePage } from './review/ReviewQueuePage';

export const screens = {
  'menu.game_ops.dashboard': GameDashboard,
  'menu.game_ops.world': configMenuScreen('world'),
  'menu.game_ops.npcs': configMenuScreen('npcs'),
  'menu.game_ops.quests': configMenuScreen('quests'),
  'menu.game_ops.minigames': configMenuScreen('minigames'),
  'menu.game_ops.challenges': configMenuScreen('challenges'),
  'menu.game_ops.bosses': configMenuScreen('bosses'),
  'menu.game_ops.progression': configMenuScreen('progression'),
  'menu.game_ops.achievements': configMenuScreen('achievements'),
  'menu.game_ops.items': configMenuScreen('items'),
  'menu.game_ops.shop': configMenuScreen('shop'),
  'menu.game_ops.rewards': configMenuScreen('rewards'),
  'menu.game_ops.events': configMenuScreen('events'),
  'menu.game_ops.competition': placeholderScreen,
  'menu.game_ops.chat_assets': placeholderScreen,
  'menu.game_ops.media': GameMediaPage,
  'menu.game_ops.translations': placeholderScreen,
  'menu.game_ops.players': placeholderScreen,
  'menu.game_ops.parties': placeholderScreen,
  'menu.game_ops.review': ReviewQueuePage,
} satisfies MenuScreens<'game_ops'>;

export default function GameOpsWorkspace() {
  return <WorkspaceRoutes workspace="game_ops" screens={screens} />;
}
