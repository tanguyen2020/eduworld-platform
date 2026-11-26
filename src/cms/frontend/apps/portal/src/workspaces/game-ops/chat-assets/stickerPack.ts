// Sticker pack payload of module chat (specs/schemas/sticker-pack.schema.json, F118; F082-AC-02): types and the display
// blockers of "Gửi duyệt" — a missing `en` name or alt text (game_text_required_languages). Display only: module chat
// checks the same at submit_review (422 TRANSLATION_INCOMPLETE, Clarifications 2026-11-26) and the server always decides.
import type { JsonSchema } from '@eduworld/ops-ui/game-console';

export interface Sticker {
  sticker_id?: string;
  asset_key: string;
  content_type?: string;
  size_bytes?: number;
  alt_text_vi?: string;
  alt_text_en?: string;
  sort_order?: number;
  preview_url?: string;
}

export interface StickerPack {
  code?: string;
  name_vi?: string;
  name_en?: string;
  unlock_rule?: { item_id?: string; reward_rule_id?: string };
  unlock_hint_vi?: string;
  unlock_hint_en?: string;
  stickers?: Sticker[];
}

export type UnlockKind = 'default' | 'reward_rule' | 'item';

export const unlockKindOf = (p: StickerPack): UnlockKind => (p.unlock_rule?.reward_rule_id ? 'reward_rule' : p.unlock_rule?.item_id ? 'item' : 'default');

export interface MissingText {
  /** DOM id of the field (jump target). */
  field: string;
  /** i18n key + params of its label. */
  labelKey: string;
  params?: Record<string, unknown>;
}

const blank = (s: string | undefined) => !s || s.trim() === '';

/** Required `en` texts still empty (the `vi` ones are required by the schema at save). */
export function missingEnglish(p: StickerPack): MissingText[] {
  const out: MissingText[] = [];
  if (blank(p.name_en)) out.push({ field: 'gf-sticker-packs-name.en', labelKey: 'gameOps.chatAssets.pack.name' });
  (p.stickers ?? []).forEach((s, i) => {
    if (blank(s.alt_text_en)) out.push({ field: `gf-sticker-packs-sticker-${i}.en`, labelKey: 'gameOps.chatAssets.pack.stickerAlt', params: { n: i + 1 } });
  });
  return out;
}

/** maxLength of a property of the pack schema (or of a sticker). */
export function packMax(schema: JsonSchema | undefined, key: string, sticker = false): number | undefined {
  const props = (sticker ? (schema?.properties?.stickers?.items as JsonSchema | undefined)?.properties : schema?.properties) ?? {};
  return (props[key] as JsonSchema | undefined)?.maxLength;
}

/** sort_order follows the order of the list. */
export const renumber = (list: Sticker[]): Sticker[] => list.map((s, i) => ({ ...s, sort_order: i }));
