// Reference fields of the game config builders (F079-AC-07, ADMIN_PORTAL §3.3): an area / NPC / scene / interactable is
// chosen from the CURRENT client manifest (getGameClientManifest — the value set submit_review checks, F078); content
// references (item, quest, NPC dialogue, reward rule, story chapter) from the PUBLISHED configs of module game (every
// page of listGameConfigs `status=published`). Never a free id: ManifestRefPicker has no tags mode. While no manifest is
// registered (`registered = false`) areas / NPCs / scenes come from the published world areas and NPC dialogues.
import { ManifestRefPicker, type RefOption, type SchemaFieldContext } from '@eduworld/ops-ui/game-console';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { useClientManifest } from '../api/console';
import { useAllConfigs, type GameConfigVersion } from '../api/gameConfigs';
import { CONFIG_TYPES, nameOf, type ConfigType } from '../registry/configTypes';

export type RefKind = 'area' | 'npc' | 'scene' | 'interactable' | 'item' | 'quest' | 'dialogue' | 'reward_rule' | 'chapter' | 'challenge' | 'shop_item' | 'collection' | 'minigame';

/** Reference kind of a payload field (by property name; `items` of a string array use the array name). */
export function refKindOf(key: string, path: readonly (string | number)[]): RefKind | undefined {
  const parent = String(path.filter((p) => typeof p === 'string').at(-2) ?? '');
  if (key === 'area_id' || key.endsWith('_area_id') || key === 'area_ids' || (key === 'areas' && parent === 'decoration')) return 'area';
  if (key === 'npc_id' || key.endsWith('_npc_id')) return 'npc';
  if (key === 'scene_ref') return 'scene';
  if (key === 'interactable_id' || key === 'interactable_ids') return 'interactable';
  if (key === 'item_id') return 'item';
  if (key === 'quest_id' || key === 'quest_ids' || key === 'main_quest_ids') return 'quest';
  if (key === 'dialogue_ids') return 'dialogue';
  if (key === 'reward_rule_id') return 'reward_rule';
  if (key === 'story_chapter_code') return 'chapter';
  if (key === 'challenge_id' || key === 'challenge_ids') return 'challenge';
  if (key === 'shop_item_ids') return 'shop_item';
  if (key === 'collection_id') return 'collection';
  if (key === 'minigame_config_id') return 'minigame';
  return undefined;
}

const SOURCE: Record<Exclude<RefKind, 'interactable'>, { type: ConfigType; value: (v: GameConfigVersion) => string | undefined }> = {
  area: { type: 'world-areas', value: (v) => v.payload.area_id as string | undefined },
  scene: { type: 'world-areas', value: (v) => v.payload.scene_ref as string | undefined },
  npc: { type: 'npc-dialogues', value: (v) => v.payload.npc_id as string | undefined },
  item: { type: 'items', value: (v) => v.payload.item_id as string | undefined },
  quest: { type: 'quests', value: (v) => v.config_id },
  dialogue: { type: 'npc-dialogues', value: (v) => v.config_id },
  reward_rule: { type: 'reward-rules', value: (v) => v.config_id },
  chapter: { type: 'story-chapters', value: (v) => v.payload.chapter_code as string | undefined },
  challenge: { type: 'challenges', value: (v) => v.config_id },
  shop_item: { type: 'shop-items', value: (v) => v.config_id },
  collection: { type: 'collections', value: (v) => v.config_id },
  minigame: { type: 'minigame-configs', value: (v) => v.config_id },
};

const MANIFEST_KINDS = new Set<RefKind>(['area', 'npc', 'scene', 'interactable']);

function useRefOptions(kind: RefKind, lang: 'vi' | 'en'): { options: RefOption[]; loading: boolean; truncated: boolean } {
  const manifest = useClientManifest();
  const useManifest = MANIFEST_KINDS.has(kind) && !!manifest.data?.registered;
  const src = kind === 'interactable' ? undefined : SOURCE[kind];
  const published = useAllConfigs(src?.type ?? 'world-areas', 'published', !!src && !useManifest && (!MANIFEST_KINDS.has(kind) || !manifest.isPending));
  if (useManifest && manifest.data) {
    const m = manifest.data;
    if (kind === 'area') return { options: m.areas.map((a) => ({ value: a.area_id, label: a.name?.[lang] })), loading: false, truncated: false };
    if (kind === 'npc') return { options: m.npcs.map((n) => ({ value: n.npc_id, label: n.name?.[lang] })), loading: false, truncated: false };
    if (kind === 'scene') return { options: m.scene_refs.map((s) => ({ value: s })), loading: false, truncated: false };
    return { options: m.interactables.map((s) => ({ value: s })), loading: false, truncated: false };
  }
  if (!src) return { options: [], loading: manifest.isPending, truncated: false };
  const def = CONFIG_TYPES[src.type];
  const seen = new Set<string>();
  const options: RefOption[] = [];
  for (const v of published.items) {
    const value = src.value(v);
    if (!value || seen.has(value)) continue;
    seen.add(value);
    options.push({ value, label: v.title ?? (nameOf(def, v.payload, lang) || undefined) });
  }
  return { options, loading: published.isPending || (MANIFEST_KINDS.has(kind) && manifest.isPending), truncated: published.truncated };
}

export function RefField({ kind, ctx }: { kind: RefKind; ctx: SchemaFieldContext }) {
  const { i18n } = useTranslation();
  const lang = i18n.language.startsWith('en') ? 'en' : 'vi';
  const { options, loading, truncated } = useRefOptions(kind, lang);
  const multiple = ctx.schema.type === 'array' || Array.isArray(ctx.value);
  return (
    <ManifestRefPicker
      id={ctx.id}
      ariaLabel={ctx.label}
      options={options}
      loading={loading}
      truncated={truncated}
      disabled={ctx.readOnly}
      multiple={multiple}
      value={ctx.value as string | string[] | undefined}
      onChange={(v) => ctx.onChange(Array.isArray(v) && v.length === 0 ? undefined : v)}
    />
  );
}

/**
 * renderOverride of SchemaForm: reference fields become pickers (manifest / published content). The item key of the
 * edited type itself (area_id of a world area, item_id of an item) is typed by the author, not picked.
 */
export function refOverrideFor(idField: string) {
  return (ctx: SchemaFieldContext): ReactNode | undefined => {
    if (ctx.path.length === 1 && ctx.key === idField) return undefined;
    const kind = refKindOf(ctx.key, ctx.path);
    return kind ? <RefField kind={kind} ctx={ctx} /> : undefined;
  };
}

