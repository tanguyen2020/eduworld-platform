// schemaValidator (F053 T017, F053-AC-14, research R2, AGENTS R33): validates the authoring parts of a question with
// the generated copy of specs/schemas/question.schema.json (ajv 2020-12) before saving — stem, explanation, hints
// (richContent), the type's render_payload / answer_key and the scoring rule — and maps each error to the dotted
// field path used by the editor (`stem.blocks.0.text`, `render_payload.options.1.feedback`), like a server
// `422 VALIDATION_FAILED` (`errors[].field`). The server validates again (F022/F023).
import Ajv2020, { type ErrorObject, type ValidateFunction } from 'ajv/dist/2020';
import type { QuestionDraft, QuestionType } from '@eduworld/ops-ui/content';
import enumsSchema from '../../../shared/api/schemas/enums.schema.json';
import questionSchema from '../../../shared/api/schemas/question.schema.json';

/** `$id` of the question schema (its `$ref`s resolve against it). */
const QUESTION_ID = (questionSchema as { $id: string }).$id;

/** question.schema.json `$defs` name of each type. */
const TYPE_DEF: Record<QuestionType, string> = {
  single_choice: 'singleChoice',
  multiple_choice: 'multipleChoice',
  true_false: 'trueFalse',
  fill_in_blank: 'fillBlank',
  short_answer: 'shortAnswer',
  numeric: 'numeric',
  matching: 'matching',
  ordering: 'ordering',
  drag_drop: 'dragDrop',
  image_based: 'imageBased',
  interactive: 'interactive',
};

export interface SchemaFieldError {
  /** Dotted field path of the editor. */
  field: string;
  /** ajv keyword: maxLength, minItems, required, pattern… */
  keyword: string;
  params: Record<string, unknown>;
}

let ajv: Ajv2020 | null = null;
const compiled = new Map<string, ValidateFunction>();

function validator(ref: string): ValidateFunction {
  if (!ajv) {
    ajv = new Ajv2020({ strict: false, allErrors: true });
    ajv.addFormat('uuid', /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i);
    ajv.addFormat('date-time', true);
    ajv.addSchema(enumsSchema as object);
    ajv.addSchema(questionSchema as object);
  }
  let v = compiled.get(ref);
  if (!v) {
    v = ajv.compile({ $ref: `${QUESTION_ID}${ref}` });
    compiled.set(ref, v);
  }
  return v;
}

/** `/blocks/0/text` under `stem` → `stem.blocks.0.text`; a missing property is reported on the property itself. */
export function toFieldPath(prefix: string, e: ErrorObject): string {
  const parts = e.instancePath.split('/').filter(Boolean).map((p) => p.replace(/~1/g, '/').replace(/~0/g, '~'));
  if (e.keyword === 'required' && typeof e.params.missingProperty === 'string') parts.push(e.params.missingProperty);
  return [prefix, ...parts].filter(Boolean).join('.');
}

// oneOf / if-then wrappers repeat the leaf errors: keep only the leaves
const NOISE = new Set(['oneOf', 'anyOf', 'if', 'not', 'propertyNames']);

function run(prefix: string, ref: string, value: unknown): SchemaFieldError[] {
  if (value === undefined) return [];
  const v = validator(ref);
  if (v(value)) return [];
  return (v.errors ?? []).filter((e) => !NOISE.has(e.keyword)).map((e) => ({ field: toFieldPath(prefix, e), keyword: e.keyword, params: e.params as Record<string, unknown> }));
}

/** Schema errors of the authoring parts of a question draft (empty = valid). */
export function validateQuestionDraft(d: QuestionDraft): SchemaFieldError[] {
  const def = `#/$defs/${TYPE_DEF[d.type]}/properties`;
  const out = [
    ...run('stem', '#/$defs/richContent', d.stem),
    ...run('explanation', '#/$defs/richContent', d.explanation),
    ...(d.hints ?? []).flatMap((h, i) => run(`hints.${i}`, '#/$defs/richContent', h)),
    ...run('render_payload', `${def}/render_payload`, d.render_payload),
    ...run('answer_key', `${def}/answer_key`, d.answer_key),
    ...run('scoring_rule', '#/$defs/scoringRule', d.scoring_rule),
  ];
  if ((d.hints?.length ?? 0) > 5) out.push({ field: 'hints', keyword: 'maxItems', params: { limit: 5 } });
  // one message per field
  const seen = new Set<string>();
  return out.filter((e) => (seen.has(e.field) ? false : (seen.add(e.field), true)));
}

/** i18n key + options of a schema error (`content_studio.validation.<keyword>`). */
export function schemaErrorMessage(e: SchemaFieldError, t: (k: string, o?: Record<string, unknown>) => string): string {
  const known = ['maxLength', 'minLength', 'maxItems', 'minItems', 'required', 'pattern', 'enum', 'type'];
  return t(`content_studio.validation.${known.includes(e.keyword) ? e.keyword : 'invalid'}`, { limit: e.params.limit ?? '' });
}
