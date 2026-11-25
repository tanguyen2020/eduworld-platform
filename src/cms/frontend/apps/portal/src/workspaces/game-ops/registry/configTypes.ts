// Config type registry of workspace A (F079 T001, T022, plan "registry/configTypes.ts"): display metadata only — which
// menu shows a `config_type`, its catalog action keys (ADR-0090), the JSON Schema copy the form is generated from
// (pnpm gen:schemas, single source specs/schemas), the builder, the payload fields the server fills, and what the
// release API offers for it (F078 Clarifications 2026-11-24: `events` has no rollback/unpublish; `reward-rules` no
// rollback; `currencies`, `progression-levels` have no type module yet → 404; `bosses` is served since F103). The server authorizes and
// validates everything again (AGENTS R1, R20).
import type { JsonSchema } from '@eduworld/ops-ui/game-console';
import type { ActionKey } from '../../../generated/menu-catalog';
import { workspaceByKey } from '../../../permissions/menuRegistry';
import type { components } from '../../../shared/api/schema.gen';
import achievement from '../../../shared/api/schemas/game/achievement.schema.json';
import boss from '../../../shared/api/schemas/game/boss.schema.json';
import challenge from '../../../shared/api/schemas/game/challenge.schema.json';
import collection from '../../../shared/api/schemas/game/collection.schema.json';
import event from '../../../shared/api/schemas/game/event.schema.json';
import eventTemplate from '../../../shared/api/schemas/game/event-template.schema.json';
import item from '../../../shared/api/schemas/game/item.schema.json';
import minigameConfig from '../../../shared/api/schemas/game/minigame-config.schema.json';
import npcDialogue from '../../../shared/api/schemas/game/npc-dialogue.schema.json';
import progressionCurve from '../../../shared/api/schemas/game/progression-curve.schema.json';
import quest from '../../../shared/api/schemas/game/quest.schema.json';
import rewardRule from '../../../shared/api/schemas/game/reward-rule.schema.json';
import seasonTrack from '../../../shared/api/schemas/game/season-track.schema.json';
import shopItem from '../../../shared/api/schemas/game/shop-item.schema.json';
import storyChapter from '../../../shared/api/schemas/game/story-chapter.schema.json';
import worldArea from '../../../shared/api/schemas/game/world-area.schema.json';

export type ConfigType = components['schemas']['GameModuleConfigType'];
export type GameOpsMenu = 'world' | 'npcs' | 'quests' | 'minigames' | 'challenges' | 'bosses' | 'progression' | 'achievements' | 'items' | 'shop' | 'rewards' | 'events';
export type BuilderKind = 'generic' | 'quest' | 'npc' | 'progression' | 'event' | 'season' | 'event-template';

export interface ConfigTypeDef {
  type: ConfigType;
  menu: GameOpsMenu;
  builder: BuilderKind;
  /** Payload schema (absent = the type has no schema yet). */
  schema?: JsonSchema;
  /** False: module game serves no type module for it yet (404) — the screen says so, no API call. */
  served: boolean;
  /** Payload field of the item key; `uuidKey` = the server sets it to the config_id. */
  idField: string;
  uuidKey: boolean;
  /** Payload fields the server fills (Normalize of F078): never edited. */
  managedKeys: string[];
  /** Payload field shown as the code column ("" = the id field). */
  codeField: string;
  /** Payload field holding the bilingual display name. */
  nameField?: string;
  rollback: boolean;
  unpublish: boolean;
  /** A 3D model is rendered on publish: thumbnail / turntable (F078-AC-20). */
  has3D: boolean;
  /** Permission to write the type (display only, ADMIN_PORTAL §2.2). */
  writePermission: string;
}

const MANAGED = ['version', 'status', 'owner_scope', 'tenant_id'];

const def = (d: Omit<ConfigTypeDef, 'managedKeys'> & { versionIdField?: string }): ConfigTypeDef => {
  const { versionIdField, ...rest } = d;
  return { ...rest, managedKeys: [...(d.uuidKey ? [d.idField] : []), ...(versionIdField ? [versionIdField] : []), ...MANAGED] };
};

export const CONFIG_TYPES: Record<ConfigType, ConfigTypeDef> = {
  'world-areas': def({ type: 'world-areas', menu: 'world', builder: 'generic', schema: worldArea as unknown as JsonSchema, served: true, idField: 'area_id', uuidKey: false, codeField: 'area_id', nameField: 'name', rollback: true, unpublish: true, has3D: true, writePermission: 'game.world.write' }),
  'story-chapters': def({ type: 'story-chapters', menu: 'quests', builder: 'generic', schema: storyChapter as unknown as JsonSchema, served: true, idField: 'chapter_code', uuidKey: false, codeField: 'chapter_code', nameField: 'title', rollback: true, unpublish: true, has3D: false, writePermission: 'game.world.write' }),
  'npc-dialogues': def({ type: 'npc-dialogues', menu: 'npcs', builder: 'npc', schema: npcDialogue as unknown as JsonSchema, served: true, idField: 'dialogue_id', uuidKey: true, versionIdField: 'dialogue_version_id', codeField: 'npc_id', nameField: 'npc_name', rollback: true, unpublish: true, has3D: true, writePermission: 'game.npc.write' }),
  quests: def({ type: 'quests', menu: 'quests', builder: 'quest', schema: quest as unknown as JsonSchema, served: true, idField: 'quest_id', uuidKey: true, versionIdField: 'quest_version_id', codeField: 'code', nameField: 'name', rollback: true, unpublish: true, has3D: true, writePermission: 'game.quest.write' }),
  'minigame-configs': def({ type: 'minigame-configs', menu: 'minigames', builder: 'generic', schema: minigameConfig as unknown as JsonSchema, served: true, idField: 'minigame_config_id', uuidKey: true, versionIdField: 'minigame_config_version_id', codeField: 'code', nameField: 'name', rollback: true, unpublish: true, has3D: false, writePermission: 'game.minigame.write' }),
  challenges: def({ type: 'challenges', menu: 'challenges', builder: 'generic', schema: challenge as unknown as JsonSchema, served: true, idField: 'challenge_id', uuidKey: true, versionIdField: 'challenge_version_id', codeField: 'code', nameField: 'name', rollback: true, unpublish: true, has3D: true, writePermission: 'game.challenge.write' }),
  bosses: def({ type: 'bosses', menu: 'bosses', builder: 'generic', schema: boss as unknown as JsonSchema, served: true, idField: 'boss_id', uuidKey: true, versionIdField: 'boss_version_id', codeField: 'code', nameField: 'name', rollback: true, unpublish: true, has3D: false, writePermission: 'game.boss.write' }),
  'progression-curves': def({ type: 'progression-curves', menu: 'progression', builder: 'progression', schema: progressionCurve as unknown as JsonSchema, served: true, idField: 'curve_id', uuidKey: true, versionIdField: 'curve_version_id', codeField: 'code', rollback: true, unpublish: true, has3D: false, writePermission: 'game.progression.write' }),
  'progression-levels': def({ type: 'progression-levels', menu: 'progression', builder: 'generic', served: false, idField: 'level', uuidKey: false, codeField: 'level', rollback: true, unpublish: true, has3D: false, writePermission: 'game.progression.write' }),
  achievements: def({ type: 'achievements', menu: 'achievements', builder: 'generic', schema: achievement as unknown as JsonSchema, served: true, idField: 'achievement_id', uuidKey: true, versionIdField: 'achievement_version_id', codeField: 'code', nameField: 'name', rollback: true, unpublish: true, has3D: false, writePermission: 'game.achievement.write' }),
  items: def({ type: 'items', menu: 'items', builder: 'generic', schema: item as unknown as JsonSchema, served: true, idField: 'item_id', uuidKey: false, versionIdField: 'item_version_id', codeField: 'item_id', nameField: 'name', rollback: true, unpublish: true, has3D: true, writePermission: 'game.item.write' }),
  collections: def({ type: 'collections', menu: 'items', builder: 'generic', schema: collection as unknown as JsonSchema, served: true, idField: 'collection_id', uuidKey: true, versionIdField: 'collection_version_id', codeField: 'code', nameField: 'title', rollback: true, unpublish: true, has3D: false, writePermission: 'game.collection.write' }),
  'shop-items': def({ type: 'shop-items', menu: 'shop', builder: 'generic', schema: shopItem as unknown as JsonSchema, served: true, idField: 'shop_item_id', uuidKey: true, versionIdField: 'shop_item_version_id', codeField: 'item_id', rollback: true, unpublish: true, has3D: false, writePermission: 'game.shop.write' }),
  currencies: def({ type: 'currencies', menu: 'shop', builder: 'generic', served: false, idField: 'currency_code', uuidKey: false, codeField: 'currency_code', rollback: true, unpublish: true, has3D: false, writePermission: 'game.economy.write' }),
  'reward-rules': def({ type: 'reward-rules', menu: 'rewards', builder: 'generic', schema: rewardRule as unknown as JsonSchema, served: true, idField: 'reward_rule_id', uuidKey: true, codeField: 'code', rollback: false, unpublish: true, has3D: false, writePermission: 'game.reward_rule.write' }),
  events: def({ type: 'events', menu: 'events', builder: 'event', schema: event as unknown as JsonSchema, served: true, idField: 'event_id', uuidKey: true, versionIdField: 'event_version_id', codeField: 'code', nameField: 'name', rollback: false, unpublish: false, has3D: false, writePermission: 'game.event.write' }),
  'event-templates': def({ type: 'event-templates', menu: 'events', builder: 'event-template', schema: eventTemplate as unknown as JsonSchema, served: true, idField: 'template_id', uuidKey: true, versionIdField: 'template_version_id', codeField: 'code', nameField: 'name', rollback: true, unpublish: true, has3D: false, writePermission: 'game.event.write' }),
  'season-tracks': def({ type: 'season-tracks', menu: 'events', builder: 'season', schema: seasonTrack as unknown as JsonSchema, served: true, idField: 'season_track_id', uuidKey: true, versionIdField: 'season_track_version_id', codeField: 'code', nameField: 'name', rollback: true, unpublish: true, has3D: false, writePermission: 'game.season_track.write' }),
};

/** Config types of a menu in display order. */
export function typesOfMenu(menu: GameOpsMenu): ConfigTypeDef[] {
  return Object.values(CONFIG_TYPES).filter((d) => d.menu === menu);
}

export type ActionKind = 'view' | 'edit' | 'workflow' | 'publish' | 'rollback';

/** Catalog action key of a type (the `x-portal-action` map of the game-admin operations). */
export function actionOf(def: ConfigTypeDef, kind: ActionKind): ActionKey {
  return `action.game_ops.${def.menu}.${kind}` as ActionKey;
}

/** Display code and name of a version payload. */
export function codeOf(def: ConfigTypeDef, payload: Record<string, unknown> | undefined): string {
  const v = payload?.[def.codeField];
  return typeof v === 'string' || typeof v === 'number' ? String(v) : '';
}

export function nameOf(def: ConfigTypeDef, payload: Record<string, unknown> | undefined, lang: 'vi' | 'en'): string {
  const v = def.nameField ? payload?.[def.nameField] : undefined;
  if (typeof v === 'string') return v;
  if (v && typeof v === 'object') return String((v as Record<string, unknown>)[lang] ?? (v as Record<string, unknown>).vi ?? '');
  return '';
}

export const isConfigType = (v: string | undefined): v is ConfigType => !!v && v in CONFIG_TYPES;

/** Route of a menu of workspace A (path of the generated catalog, ADR-0090). */
export function menuPathOf(menu: GameOpsMenu): string {
  return workspaceByKey('game_ops').menus.find((m) => m.key === `menu.game_ops.${menu}`)!.path;
}

/** Editor route of one version. */
export const editorPathOf = (type: ConfigType, configId: string, version: number) => `${menuPathOf(CONFIG_TYPES[type].menu)}/${type}/${configId}/v/${version}`;
