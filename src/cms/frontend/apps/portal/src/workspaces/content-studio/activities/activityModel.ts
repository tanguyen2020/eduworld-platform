// activityModel (F058 US1, ACTIVITY_ENGINE §4, pure): the form state of the Activity Builder, its mapping to
// ActivityDefinitionInput / from ActivityDefinition, and the client-side schema check of `config` with the generated
// copy of specs/schemas/activities/<type>.schema.json (ajv 2020-12) — errors keyed by the JSON pointer the server uses
// for 422 ACTIVITY_CONFIG_INVALID (`/config/content/…`). An early warning only: module learning validates again.
import Ajv2020, { type ErrorObject, type ValidateFunction } from 'ajv/dist/2020';
import type { components } from '../../../shared/api/schema.gen';
import enumsSchema from '../../../shared/api/schemas/enums.schema.json';
import baseSchema from '../../../shared/api/schemas/activities/activity-base.schema.json';
import quizSchema from '../../../shared/api/schemas/activities/quiz.schema.json';
import interactiveSchema from '../../../shared/api/schemas/activities/interactive-exercise.schema.json';
import videoSchema from '../../../shared/api/schemas/activities/video-lesson.schema.json';
import audioSchema from '../../../shared/api/schemas/activities/audio-lesson.schema.json';
import simulationSchema from '../../../shared/api/schemas/activities/simulation.schema.json';

export type ActivityDefinition = components['schemas']['ActivityDefinition'];
export type ActivityDefinitionInput = components['schemas']['ActivityDefinitionInput'];
export type ActivityType = components['schemas']['ActivityType'];
export type ActivityTypeInfo = components['schemas']['ActivityTypeInfo'];
export type LangText = { vi?: string; en?: string };
export type Config = Record<string, unknown>;

export interface ActivityState {
  activity_type: ActivityType;
  title: LangText;
  description: LangText;
  instructions: LangText;
  content_language: 'vi' | 'en';
  subject_id?: string;
  grade_levels: number[];
  topic_ids: string[];
  objective_ids: string[];
  difficulty?: number;
  time_limit_seconds?: number;
  config: Config;
  math_assets: NonNullable<ActivityDefinitionInput['math_assets']>;
  /** Voice-over of the instructions, required for grades 1–3 unless exempted (CONTENT_MODEL §8.1, F058-AC-09). */
  instructions_audio_ref?: string;
  audio_exempt_reason?: string;
}

const SCHEMAS: Partial<Record<ActivityType, { $id: string }>> = {
  quiz: quizSchema as { $id: string },
  interactive_exercise: interactiveSchema as { $id: string },
  video_lesson: videoSchema as { $id: string },
  audio_lesson: audioSchema as { $id: string },
  simulation: simulationSchema as { $id: string },
};

export const emptyActivity = (type: ActivityType, config: Config): ActivityState => ({
  activity_type: type,
  title: {},
  description: {},
  instructions: {},
  content_language: 'vi',
  grade_levels: [],
  topic_ids: [],
  objective_ids: [],
  difficulty: 2,
  config,
  math_assets: [],
});

const lang = (v: unknown): LangText => (v && typeof v === 'object' ? (v as LangText) : {});
const nonEmpty = (v: LangText) => (Object.values(v).some((s) => (s ?? '').trim() !== '') ? Object.fromEntries(Object.entries(v).filter(([, s]) => (s ?? '').trim() !== '')) : undefined);

export function fromDefinition(d: ActivityDefinition): ActivityState {
  return {
    activity_type: d.activity_type,
    title: lang(d.title),
    description: lang(d.description),
    instructions: lang(d.instructions),
    content_language: d.content_language === 'en' ? 'en' : 'vi',
    subject_id: d.subject_id,
    grade_levels: d.grade_levels ?? [],
    topic_ids: d.topic_id ? [d.topic_id] : [],
    objective_ids: d.objective_ids ?? [],
    difficulty: d.difficulty,
    time_limit_seconds: d.time_limit_seconds,
    config: (d.config as Config) ?? {},
    math_assets: (d.math_assets ?? []) as ActivityState['math_assets'],
    instructions_audio_ref: d.instructions_audio_ref,
    audio_exempt_reason: d.audio_exempt_reason,
  };
}

/** ActivityDefinitionInput of the form (full body: updateCmsActivityDraft replaces the draft). */
export function toInput(s: ActivityState): ActivityDefinitionInput {
  const out: Record<string, unknown> = {
    activity_type: s.activity_type,
    title: nonEmpty(s.title) ?? {},
    grade_levels: s.grade_levels,
    subject_id: s.subject_id,
    objective_ids: s.objective_ids,
    content_language: s.content_language,
    config: s.config,
    math_assets: s.math_assets,
  };
  if (nonEmpty(s.description)) out.description = nonEmpty(s.description);
  if (nonEmpty(s.instructions)) out.instructions = nonEmpty(s.instructions);
  if (s.topic_ids[0]) out.topic_id = s.topic_ids[0];
  if (s.difficulty) out.difficulty = s.difficulty;
  if (s.time_limit_seconds) out.time_limit_seconds = s.time_limit_seconds;
  if (s.instructions_audio_ref) out.instructions_audio_ref = s.instructions_audio_ref;
  // an empty exemption clears the stored one (merge semantics of the draft PATCH)
  out.audio_exempt_reason = s.audio_exempt_reason?.trim() ?? '';
  return out as ActivityDefinitionInput;
}

let ajv: Ajv2020 | null = null;
const compiled = new Map<string, ValidateFunction>();

function validatorFor(type: ActivityType): ValidateFunction | null {
  const schema = SCHEMAS[type];
  if (!schema) return null;
  if (!ajv) {
    ajv = new Ajv2020({ strict: false, allErrors: true });
    ajv.addFormat('uuid', /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i);
    ajv.addFormat('date-time', true);
    ajv.addFormat('uri', true);
    for (const s of [enumsSchema, baseSchema, ...Object.values(SCHEMAS)]) ajv.addSchema(s as object);
  }
  let v = compiled.get(type);
  if (!v) {
    const base = (baseSchema as { $id: string }).$id;
    // The config of ActivityDefinitionInput, checked with the definitions of the type schema (same wrapper as the server).
    v = ajv.compile({
      type: 'object',
      required: ['content', 'evaluation', 'feedback', 'completion_rule'],
      properties: {
        content: { $ref: `${schema.$id}#/properties/content` },
        interaction: { $ref: `${schema.$id}#/properties/interaction` },
        evaluation: { $ref: `${base}#/$defs/evaluation` },
        feedback: { $ref: `${base}#/$defs/feedback` },
        completion_rule: { $ref: `${base}#/$defs/completionRule` },
      },
      additionalProperties: false,
    });
    compiled.set(type, v);
  }
  return v;
}

export interface ConfigError {
  /** JSON pointer under /config (the field key of a server 422). */
  pointer: string;
  keyword: string;
  params: Record<string, unknown>;
}

const NOISE = new Set(['oneOf', 'anyOf', 'if', 'not', 'allOf']);

function pointerOf(e: ErrorObject): string {
  const missing = e.keyword === 'required' && typeof e.params.missingProperty === 'string' ? `/${e.params.missingProperty}` : '';
  return `/config${e.instancePath}${missing}`;
}

/** Schema errors of the config of a type (empty = valid or no schema for the type). */
export function validateConfig(type: ActivityType, config: Config): ConfigError[] {
  const v = validatorFor(type);
  if (!v || v(config)) return [];
  const seen = new Set<string>();
  return (v.errors ?? [])
    .filter((e) => !NOISE.has(e.keyword))
    .map((e) => ({ pointer: pointerOf(e), keyword: e.keyword, params: e.params as Record<string, unknown> }))
    .filter((e) => (seen.has(e.pointer) ? false : (seen.add(e.pointer), true)));
}

/** Metadata rules of ActivityDefinitionInput (F058-AC-02): title, grades, subject, 1–20 LOs, difficulty 1–5. */
export function metaErrors(s: ActivityState): string[] {
  const out: string[] = [];
  if (!(s.title[s.content_language] ?? '').trim()) out.push('/title');
  if (s.grade_levels.length === 0) out.push('/grade_levels');
  if (!s.subject_id) out.push('/subject_id');
  if (s.objective_ids.length === 0 || s.objective_ids.length > 20) out.push('/objective_ids');
  if (s.difficulty !== undefined && (s.difficulty < 1 || s.difficulty > 5)) out.push('/difficulty');
  return out;
}

type ContentSchema = { properties?: { content?: { properties?: { allowed_question_types?: { default?: string[]; items?: { enum?: string[] } } } } } };

/** `allowed_question_types` of a question-based type: the default and the possible values of its schema. */
export function allowedTypesOf(type: ActivityType): { defaults: string[]; values: string[] } {
  const p = (SCHEMAS[type] as ContentSchema | undefined)?.properties?.content?.properties?.allowed_question_types;
  return { defaults: [...(p?.default ?? [])], values: [...(p?.items?.enum ?? p?.default ?? [])] };
}

/** FeedbackTiming enum (specs/schemas/common/enums.schema.json). */
export const FEEDBACK_TIMINGS = [...((enumsSchema as { $defs: { FeedbackTiming: { enum: string[] } } }).$defs.FeedbackTiming.enum)];

/** completion_rule.mode enum (activity-base.schema.json). */
export const COMPLETION_MODES = [...((baseSchema as { $defs: { completionRule: { properties: { mode: { enum: string[] } } } } }).$defs.completionRule.properties.mode.enum)];
