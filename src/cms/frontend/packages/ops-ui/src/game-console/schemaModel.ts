// schemaModel (F079-AC-02, AC-05): pure helpers of SchemaForm — the subset of JSON Schema 2020-12 the game config
// schemas use ($ref to $defs, allOf, enum, const, object/array/string/number/boolean), immutable path updates and the
// canonical form of a server field path (`steps.0.area_id`, `/steps/0/area_id`, `steps[0].area_id` → `steps/0/area_id`).
// Display only: the server validates every payload again (AGENTS R1).

export interface JsonSchema {
  $ref?: string;
  type?: string | string[];
  title?: string;
  description?: string;
  enum?: readonly unknown[];
  const?: unknown;
  default?: unknown;
  format?: string;
  pattern?: string;
  minLength?: number;
  maxLength?: number;
  minimum?: number;
  maximum?: number;
  minItems?: number;
  maxItems?: number;
  required?: readonly string[];
  properties?: Record<string, JsonSchema>;
  additionalProperties?: boolean | JsonSchema;
  items?: JsonSchema;
  allOf?: readonly JsonSchema[];
  oneOf?: readonly JsonSchema[];
  anyOf?: readonly JsonSchema[];
  if?: JsonSchema;
  then?: JsonSchema;
  else?: JsonSchema;
  $defs?: Record<string, JsonSchema>;
  definitions?: Record<string, JsonSchema>;
  [key: string]: unknown;
}

export type PathSegment = string | number;

/** Follows local `$ref`s (`#/$defs/x`, `#/definitions/x`) against the root schema. */
export function resolveRef(schema: JsonSchema, root: JsonSchema): JsonSchema {
  let s = schema;
  for (let i = 0; i < 16 && s.$ref; i++) {
    const parts = s.$ref.replace(/^#\//, '').split('/');
    let node: unknown = root;
    for (const p of parts) node = (node as Record<string, unknown> | undefined)?.[decodeURIComponent(p)];
    if (!node || typeof node !== 'object') return {};
    const rest: JsonSchema = { ...s };
    delete rest.$ref;
    s = { ...(node as JsonSchema), ...rest };
  }
  return s;
}

/**
 * Effective schema of a node: `$ref` resolved, plain `allOf` members merged (properties + required). Conditional
 * branches (`if/then`, `oneOf`) only add their properties as optional fields — the server checks the conditions.
 */
export function effective(schema: JsonSchema | undefined, root: JsonSchema): JsonSchema {
  if (!schema) return {};
  let s = resolveRef(schema, root);
  const extra: JsonSchema[] = [];
  for (const part of s.allOf ?? []) {
    const r = resolveRef(part, root);
    if (r.if) {
      if (r.then) extra.push({ properties: resolveRef(r.then, root).properties });
      continue;
    }
    extra.push(effective(r, root));
  }
  for (const part of [...(s.oneOf ?? []), ...(s.anyOf ?? [])]) extra.push({ properties: effective(part, root).properties });
  if (s.if && s.then) extra.push({ properties: resolveRef(s.then, root).properties });
  if (extra.length) {
    const properties: Record<string, JsonSchema> = { ...(s.properties ?? {}) };
    const required = new Set(s.required ?? []);
    for (const e of extra) {
      for (const [k, v] of Object.entries(e.properties ?? {})) {
        // `true` placeholders of conditional branches (`"amount": true`) keep the base definition
        if (typeof v === 'object' && v !== null && (!properties[k] || Object.keys(properties[k]!).length === 0)) properties[k] = v;
      }
      for (const r of e.required ?? []) required.add(r);
      if (!s.type && e.type) s = { ...s, type: e.type };
    }
    s = { ...s, properties, required: [...required] };
  }
  return s;
}

/** First non-null JSON type of a schema (`["string","null"]` → string). */
export function typeOf(schema: JsonSchema): string | undefined {
  if (Array.isArray(schema.type)) return schema.type.find((t) => t !== 'null');
  if (typeof schema.type === 'string') return schema.type;
  if (schema.properties) return 'object';
  if (schema.items) return 'array';
  if (schema.enum) return 'enum';
  return undefined;
}

/**
 * A `{vi, en}` text object (LocalizedText / localized_text of the game schemas, ADR-0056): properties `vi` (+ `en`), or
 * `vi` plus language keys in `additionalProperties` (event.schema.json).
 */
export function isLocalized(schema: JsonSchema): boolean {
  const p = schema.properties;
  if (!p || typeOf(schema) !== 'object') return false;
  const keys = Object.keys(p);
  return keys.includes('vi') && keys.every((k) => k === 'vi' || k === 'en');
}

/** Schema of one language of a localized text object. */
export function languageSchema(schema: JsonSchema, lang: string): JsonSchema {
  const raw = schema.properties?.[lang] as unknown;
  if (raw && typeof raw === 'object') return raw as JsonSchema;
  if (schema.additionalProperties && typeof schema.additionalProperties === 'object') return schema.additionalProperties;
  return { type: 'string' };
}

/** Canonical field path of a server error: segments joined by `/`, leading `payload` dropped. */
export function canonicalPath(field: string): string {
  const parts = field
    .replace(/\[(\d+)\]/g, '/$1')
    .split(/[./]/)
    .filter((p) => p !== '');
  if (parts[0] === 'payload') parts.shift();
  return parts.join('/');
}

export const pathKey = (path: readonly PathSegment[]): string => path.join('/');

/** Value at a path (undefined when absent). */
export function getIn(value: unknown, path: readonly PathSegment[]): unknown {
  let node = value;
  for (const p of path) {
    if (node === null || typeof node !== 'object') return undefined;
    node = (node as Record<string | number, unknown>)[p];
  }
  return node;
}

/** Immutable write; `undefined` removes an object key. Numeric segments create arrays. */
export function setIn(value: unknown, path: readonly PathSegment[], next: unknown): unknown {
  if (path.length === 0) return next;
  const [head, ...rest] = path;
  if (typeof head === 'number') {
    const arr = Array.isArray(value) ? [...value] : [];
    arr[head] = setIn(arr[head], rest, next);
    return arr;
  }
  const obj: Record<string, unknown> = value && typeof value === 'object' && !Array.isArray(value) ? { ...(value as Record<string, unknown>) } : {};
  const v = setIn(obj[head!], rest, next);
  if (v === undefined) delete obj[head!];
  else obj[head!] = v;
  return obj;
}

/** New value of a node: const, default, or (for objects) the const/default of its properties. */
export function initialValue(schema: JsonSchema, root: JsonSchema = schema): unknown {
  const s = effective(schema, root);
  if (s.const !== undefined) return s.const;
  if (s.default !== undefined) return s.default;
  if (typeOf(s) === 'object') {
    const out: Record<string, unknown> = {};
    for (const [k, p] of Object.entries(s.properties ?? {})) {
      const r = effective(p, root);
      if (r.const !== undefined) out[k] = r.const;
    }
    return out;
  }
  return undefined;
}

/** Fills the `const` fields a value lacks (schema_version, paid_track = false…) at every object level it holds. */
export function withConsts(value: unknown, schema: JsonSchema, root: JsonSchema = schema): unknown {
  const s = effective(schema, root);
  if (typeOf(s) === 'object' && value && typeof value === 'object' && !Array.isArray(value)) {
    let out = value as Record<string, unknown>;
    for (const [k, p] of Object.entries(s.properties ?? {})) {
      const r = effective(p, root);
      if (r.const !== undefined && out[k] === undefined) out = { ...out, [k]: r.const };
      else if (out[k] !== undefined) {
        const v = withConsts(out[k], p, root);
        if (v !== out[k]) out = { ...out, [k]: v };
      }
    }
    return out;
  }
  if (typeOf(s) === 'array' && Array.isArray(value) && s.items) {
    const next = value.map((v) => withConsts(v, s.items!, root));
    return next.some((v, i) => v !== value[i]) ? next : value;
  }
  return value;
}
