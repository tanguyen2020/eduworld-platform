// GIF payload of module chat (specs/schemas/gif.schema.json, F118; F082-AC-05): types and the schema bounds of the tags.
import type { JsonSchema } from '@eduworld/ops-ui/game-console';

export interface GifPayload {
  asset_key?: string;
  preview_key?: string;
  size_bytes?: number;
  width?: number;
  height?: number;
  tags_vi?: string[];
  tags_en?: string[];
  /** Signed short-lived URLs added by module chat to the response (never stored). */
  preview_url?: string;
  url?: string;
}

export interface TagRules {
  maxLength: number;
  maxItems: number;
  pattern?: string;
}

/** Bounds of a tag list of the GIF schema (single source: specs/schemas/gif.schema.json). */
export function tagRules(schema: JsonSchema | undefined): TagRules {
  const list = schema?.properties?.tags_vi;
  const item = list?.items as JsonSchema | undefined;
  return { maxLength: item?.maxLength ?? 0, maxItems: list?.maxItems ?? 0, pattern: item?.pattern };
}

/** Accessible name of a GIF (its tags). */
export const gifTags = (p: GifPayload): string => [...(p.tags_vi ?? []), ...(p.tags_en ?? [])].join(', ');
