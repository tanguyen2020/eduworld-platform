// @vitest-environment node
// F057 T043 (F057-AC-21, ADR-0041, AGENTS R33) + T003: every input limit of workspace D equals the schema field it
// names in specs/openapi/eduworld-api.v1.yaml; the low-days warning equals CONFIG_DEFAULTS; translations vi/en of the
// workspace have the same keys and none is empty; clone types never include student data (AGENTS R22).
import fs from 'node:fs';
import path from 'node:path';
import yaml from 'js-yaml';
import { describe, expect, it } from 'vitest';
import { CLONABLE_TYPES } from './api/clones';
import { ASSIGNABLE_TYPES } from './api/assignments';
import { TRASH_TYPES } from './api/trash';
import en from './i18n/en.json';
import vi from './i18n/vi.json';
import { CLEANUP_LOW_DAYS_WARNING, LIMITS, PAGE_SIZE } from './limits';

const REPO = path.resolve(__dirname, '../../../../../../../..');
type Node = Record<string, unknown>;
const spec = yaml.load(fs.readFileSync(path.join(REPO, 'specs/openapi/eduworld-api.v1.yaml'), 'utf8')) as Node;

const deref = (n: Node): Node => {
  if (typeof n.$ref !== 'string') return n;
  return deref(n.$ref.slice(2).split('/').reduce<Node>((o, k) => o[k] as Node, spec));
};

/** Resolves a `source` pointer of limits.ts to its schema. */
function schemaAt(source: string): Node {
  if (source.startsWith('components.parameters.')) {
    const param = deref(((spec.components as Node).parameters as Node)[source.split('.')[2]!] as Node);
    return deref(param.schema as Node);
  }
  if (source.startsWith('components.schemas.')) {
    const [, , schema, , field] = source.split('.');
    return deref(((deref(((spec.components as Node).schemas as Node)[schema!] as Node).properties as Node)[field!]) as Node);
  }
  const m = /^paths\.(.+)\.(get|post|put|patch|delete)\.(parameters|requestBody)\.(.+)$/.exec(source)!;
  const op = ((spec.paths as Node)[m[1]!] as Node)[m[2]!] as Node;
  if (m[3] === 'parameters') {
    const p = (op.parameters as Node[]).map(deref).find((x) => x.name === m[4])!;
    return deref(p.schema as Node);
  }
  const body = deref((((op.requestBody as Node).content as Node)['application/json'] as Node).schema as Node);
  return deref((body.properties as Node)[m[4]!] as Node);
}

describe('limits of workspace D = OpenAPI (F057-AC-21)', () => {
  it.each(Object.entries(LIMITS))('%s', (_name, limit) => {
    const s = schemaAt(limit.source);
    const variants = Array.isArray(s.oneOf) ? (s.oneOf as Node[]).map(deref) : [s];
    const schema = variants.find((v) => v.type !== 'null') ?? s;
    for (const k of ['minLength', 'maxLength', 'pattern', 'minimum', 'maximum', 'maxItems'] as const) {
      if (k in limit) expect(schema[k], `${limit.source} ${k}`).toEqual((limit as Record<string, unknown>)[k]);
    }
  });

  it('page size within the Limit parameter', () => {
    expect(PAGE_SIZE).toBeLessThanOrEqual(LIMITS.pageSize.maximum);
  });

  it('CONFIG_DEFAULTS inactive_student_cleanup_low_days_warning', () => {
    const doc = fs.readFileSync(path.join(REPO, 'docs/00-overview/CONFIG_DEFAULTS.md'), 'utf8');
    const row = doc.split('\n').find((l) => l.startsWith('| `inactive_student_cleanup_low_days_warning`'))!;
    expect(row).toContain(`< ${CLEANUP_LOW_DAYS_WARNING}`);
  });
});

describe('resource types (F057-AC-08, AGENTS R22)', () => {
  const enumOf = ((spec.components as Node).schemas as Node).ResourceType as { enum: string[] };

  it('every listed type is a ResourceType; the trash lists them all', () => {
    for (const x of [...CLONABLE_TYPES, ...ASSIGNABLE_TYPES, ...TRASH_TYPES]) expect(enumOf.enum).toContain(x);
    expect([...TRASH_TYPES].sort()).toEqual([...enumOf.enum].sort());
  });

  it('the clone wizard offers no student data type', () => {
    for (const x of ['user', 'student', 'enrollment', 'attempt', 'result', 'chat', 'mastery', 'xp']) expect(CLONABLE_TYPES as readonly string[]).not.toContain(x);
  });
});

describe('translations of workspace D (T003)', () => {
  const keys = (o: unknown, p = ''): string[] =>
    o && typeof o === 'object' ? Object.entries(o as Node).flatMap(([k, v]) => keys(v, p ? `${p}.${k}` : k)) : [p];
  const value = (o: unknown, k: string) => k.split('.').reduce<unknown>((n, s) => (n as Node)?.[s], o);

  it('vi and en have the same keys, none empty', () => {
    expect(keys(en).sort()).toEqual(keys(vi).sort());
    for (const k of keys(vi)) {
      expect(String(value(vi, k)).trim(), `vi ${k}`).not.toBe('');
      expect(String(value(en, k)).trim(), `en ${k}`).not.toBe('');
    }
  });

  it('keys sit directly under `platform.*` (no nested `platform` root)', () => {
    expect('platform' in vi).toBe(false);
    expect('platform' in en).toBe(false);
  });

  it('every resource type has a label', () => {
    for (const x of TRASH_TYPES) expect(value(vi, `resourceType.${x}`), x).toBeTruthy();
  });
});
