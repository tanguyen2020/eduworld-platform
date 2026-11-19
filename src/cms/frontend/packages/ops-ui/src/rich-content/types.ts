// Rich content of learning items (question.schema.json `$defs/richContent`, `richBlock`; CONTENT_MODEL §9.2 for
// `worked_example`). Structured blocks only — never free HTML (F053 T013).
import type { I18nTextValue } from '../i18n-field/translationStatus';

export type RichBlockType = 'paragraph' | 'heading' | 'list' | 'math' | 'image' | 'audio' | 'video' | 'table' | 'callout';

export interface RichBlock {
  type: RichBlockType;
  text?: I18nTextValue;
  /** Block formula (`math`), maxLength 1000 (richBlock.latex). */
  latex?: string;
  media_ref?: string;
  /** Alternative text of an image (required, maxLength 1000). */
  alt?: string;
  items?: I18nTextValue[];
  rows?: I18nTextValue[][];
  level?: number;
}

export interface RichContent {
  blocks: RichBlock[];
  /** Spoken stem (grades 1–3, CONTENT_MODEL §8.1). */
  audio_ref?: string;
}

export interface WorkedExampleStep {
  content: RichContent;
  audio_ref?: string | null;
}

/** `worked_example` block of a lesson / learning material (CONTENT_MODEL §9.2). */
export interface WorkedExample {
  type: 'worked_example';
  id: string;
  objective_ids?: string[];
  problem: RichContent;
  steps: WorkedExampleStep[];
  self_explain_prompt?: RichContent;
}

/** Limits of question.schema.json richBlock / i18nText. */
export const RICH_TEXT_MAX = 1000;
export const RICH_LATEX_MAX = 1000;
export const RICH_ALT_MAX = 1000;
export const WORKED_EXAMPLE_STEPS_MIN = 1;
export const WORKED_EXAMPLE_STEPS_MAX = 8;

export const BASIC_BLOCKS: readonly RichBlockType[] = ['paragraph', 'heading', 'list', 'math', 'image', 'audio', 'table', 'callout'];

export function emptyBlock(type: RichBlockType): RichBlock {
  switch (type) {
    case 'heading':
      return { type, level: 2 };
    case 'list':
      return { type, items: [] };
    case 'table':
      return { type, rows: [['', '']] };
    case 'math':
      return { type, latex: '' };
    default:
      return { type };
  }
}

/** Rich content with one empty paragraph (a new field). */
export const emptyRichContent = (): RichContent => ({ blocks: [{ type: 'paragraph' }] });
