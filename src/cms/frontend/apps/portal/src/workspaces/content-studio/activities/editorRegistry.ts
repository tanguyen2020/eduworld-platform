// editorRegistry (F058 T021, ACTIVITY_ENGINE §12, ADR-0048): `activity_type` → the editor of its config in the
// Activity Builder. F058 registers quiz, interactive_exercise and video_lesson; F066 plugs audio_lesson and simulation
// in with `registerActivityEditor` — the builder never switches on a type. A type of `listActivityTypes` without an
// editor here is not offered (F058-AC-01); `written_response`, `speaking`, `project` are never offered.
import type { EditorBlocker } from '@eduworld/ops-ui/content';
import type { ComponentType } from 'react';
import type { ActivityType, ActivityTypeInfo, Config } from './activityModel';

export interface ActivityEditorProps {
  config: Config;
  onChange: (c: Config) => void;
  readOnly: boolean;
  /** Errors of the last server answer or of the schema check, by JSON pointer under /config. */
  errors: Readonly<Record<string, string>>;
  typeInfo?: ActivityTypeInfo;
  /** Grade levels of the activity (video length warning by grade, F058-AC-20). */
  gradeLevels: number[];
  /** Subject of the activity (question selection by topic / LO). */
  subjectId?: string;
  /** Pipeline status of a media asset reported by the editor (blocks Gửi duyệt until ready, F058-AC-15). */
  onMediaStatus?: (assetId: string, status: string) => void;
  /** Form-only flags of the page (not saved), e.g. "video có lời" (F058 Assumptions). */
  flags: Readonly<Record<string, boolean>>;
  onFlag: (name: string, value: boolean) => void;
}

/** Context of the local blockers of an editor (asset status known to the page). */
export interface BlockerContext {
  t: (k: string, o?: Record<string, unknown>) => string;
  mediaStatus: Readonly<Record<string, string>>;
  flags: Readonly<Record<string, boolean>>;
}

export interface ActivityEditorDef {
  type: ActivityType;
  /** i18n key of the type name. */
  labelKey: string;
  /** Config of a new activity (defaults of the type, e.g. video_completion). */
  empty: () => Config;
  Editor: ComponentType<ActivityEditorProps>;
  /** Local blockers of Gửi duyệt (pure rules of the type, F058-AC-15…AC-19). */
  blockers?: (config: Config, ctx: BlockerContext) => EditorBlocker[];
}

/** Types outside the release: never offered even when a registry lists them (ADR-0072, ADR-0082). */
export const NEVER_OFFERED = ['written_response', 'speaking', 'project'] as const;

const registry = new Map<string, ActivityEditorDef>();

export function registerActivityEditor(def: ActivityEditorDef): void {
  if ((NEVER_OFFERED as readonly string[]).includes(def.type)) throw new Error(`activity type ${def.type} is outside the release`);
  registry.set(def.type, def);
}

export function activityEditor(type: string | undefined): ActivityEditorDef | undefined {
  return type ? registry.get(type) : undefined;
}

/** Types offered at step 1 (F058-AC-01): registered in the backend, not disabled, with an editor in this portal. */
export function offeredTypes(infos: readonly ActivityTypeInfo[]): ActivityTypeInfo[] {
  return infos.filter((i) => i.status !== 'disabled' && registry.has(i.activity_type) && !(NEVER_OFFERED as readonly string[]).includes(i.activity_type));
}
