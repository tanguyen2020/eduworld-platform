// F108 T027, T028 (F108-AC-14; ADR-0041, ADR-0095): limits and enums of the challenge wizard mirror the OpenAPI spec;
// the createChallenge body carries one grade target, the added students, the exclude list and only the allowed
// parameters (never a class, never tenant_id for a tenant user); server error pointers map back to wizard fields;
// vi and en translations of the screen have the same keys.
import fs from 'node:fs';
import path from 'node:path';
import yaml from 'js-yaml';
import { describe, expect, it } from 'vitest';
import en from '../../../../i18n/lms.en.json';
import vi from '../../../../i18n/lms.vi.json';
import {
  CHALLENGE_ENUMS,
  CHALLENGE_FIELDS,
  CHALLENGE_LIMITS,
  EMPTY_CHALLENGE_FORM,
  excludedIncludes,
  fieldOf,
  gradeOptions,
  paramInput,
  stepOf,
  toCreatePayload,
  validateChallenge,
  type ChallengeFormValues,
} from '../model/challengeForm';

type Node = Record<string, unknown>;
const REPO = path.resolve(__dirname, '../../../../../../../../../..');
const spec = yaml.load(fs.readFileSync(path.join(REPO, 'specs/openapi/eduworld-api.v1.yaml'), 'utf8')) as Node;

function deref(n: Node): Node {
  let cur = n;
  while (typeof cur.$ref === 'string') cur = (cur.$ref as string).slice(2).split('/').reduce<Node>((o, k) => o[k] as Node, spec);
  if (Array.isArray(cur.allOf)) return { ...cur, properties: Object.assign({}, ...(cur.allOf as Node[]).map((s) => deref(s).properties)) };
  return cur;
}

/** `schemas/A/properties/b/items/properties/c` (allOf merged). */
function at(source: string): Node {
  const [, name, ...rest] = source.split('/');
  let cur = deref(((spec.components as Node).schemas as Node)[name!] as Node);
  for (let i = 0; i < rest.length; ) {
    if (rest[i] === 'items') {
      cur = deref(cur.items as Node);
      i += 1;
    } else {
      cur = deref((cur[rest[i]!] as Node)[rest[i + 1]!] as Node);
      i += 2;
    }
  }
  return cur;
}

const keys = (o: unknown, p = ''): string[] => (o && typeof o === 'object' ? Object.entries(o as Node).flatMap(([k, v]) => keys(v, p ? `${p}.${k}` : k)) : [p]);

describe('limits mirror the OpenAPI spec (ADR-0041, AGENTS R33)', () => {
  for (const [name, f] of Object.entries(CHALLENGE_FIELDS)) {
    it(`field ${name}`, () => {
      const s = at(f.source);
      expect(s.maxLength, name).toBe(f.maxLength);
      expect(s.minLength, name).toBe('minLength' in f ? f.minLength : undefined);
    });
  }
  for (const [name, l] of Object.entries(CHALLENGE_LIMITS)) {
    it(`limit ${name}`, () => {
      const s = at(l.source);
      for (const k of ['maxItems', 'maxProperties', 'minimum', 'maximum'] as const) if (k in l) expect(s[k], `${name}.${k}`).toBe((l as unknown as Record<string, number>)[k]);
    });
  }
  for (const [name, e] of Object.entries(CHALLENGE_ENUMS)) {
    it(`enum ${name}`, () => expect(at(e.source).enum).toEqual(e.values));
  }
});

const VALID: ChallengeFormValues = {
  ...EMPTY_CHALLENGE_FORM,
  templateId: 't1',
  name: '  Giải Toán khối 3 ',
  startsAt: '2026-12-01T01:00:00.000Z',
  params: { max_entries: 32, seeding_policy: undefined, auto_confirm: false, reward: 'x', registration_close_at: '' },
  schoolId: 's1',
  grade: 3,
  includeIds: ['c4'],
  excludeIds: ['b3'],
};

describe('challenge form model', () => {
  it('payload: grade target + students target, exclude list, only allowed non-empty params, trimmed name, no tenant_id', () => {
    const body = toCreatePayload(VALID, { allowed_params: ['max_entries', 'seeding_policy', 'auto_confirm', 'registration_close_at'] });
    expect(body).toEqual({
      template_id: 't1',
      name: 'Giải Toán khối 3',
      school_id: 's1',
      starts_at: '2026-12-01T01:00:00.000Z',
      targets: [
        { target_type: 'grade', grade_level: 3 },
        { target_type: 'students', student_ids: ['c4'] },
      ],
      exclude_student_ids: ['b3'],
      params: { max_entries: 32, auto_confirm: false },
    });
    const minimal = toCreatePayload({ ...VALID, includeIds: [], excludeIds: [], params: {} }, { allowed_params: [] }, 'tenant-a');
    expect(minimal.targets).toEqual([{ target_type: 'grade', grade_level: 3 }]);
    expect(minimal).not.toHaveProperty('exclude_student_ids');
    expect(minimal).not.toHaveProperty('params');
    expect(minimal.tenant_id).toBe('tenant-a');
  });

  it('shape checks: required fields, name maxLength, list maxItems', () => {
    expect(validateChallenge(VALID)).toEqual([]);
    const many = (n: number) => Array.from({ length: n }, (_, i) => `s${i}`);
    const issues = validateChallenge({ ...EMPTY_CHALLENGE_FORM, name: 'x'.repeat(81), params: { theme_code: 'y'.repeat(256) }, includeIds: many(501), excludeIds: many(501) });
    expect(issues.map((i) => `${i.field}:${i.key}`).sort()).toEqual(
      ['templateId:required', 'name:tooLong', 'startsAt:required', 'params.theme_code:tooLong', 'schoolId:required', 'grade:required', 'includeIds:tooMany', 'excludeIds:tooMany'].sort(),
    );
  });

  it('server pointers → fields and steps', () => {
    const sent = toCreatePayload(VALID, { allowed_params: ['max_entries'] });
    expect(fieldOf('/params/max_entries', sent)).toBe('params.max_entries');
    expect(fieldOf('/targets/1/student_ids/0', sent)).toBe('includeIds');
    expect(fieldOf('/targets/0/grade_level', sent)).toBe('grade');
    expect(fieldOf('/exclude_student_ids/3', sent)).toBe('excludeIds');
    expect(fieldOf('name', sent)).toBe('name');
    expect(fieldOf('/template_id', sent)).toBe('templateId');
    expect(fieldOf('/unknown', sent)).toBeUndefined();
    expect([stepOf('templateId'), stepOf('name'), stepOf('params.max_entries'), stepOf('startsAt'), stepOf('grade'), stepOf('excludeIds')]).toEqual([0, 1, 1, 1, 2, 2]);
  });

  it('grades = school grades within the template grade_levels; exclude-wins hint; parameter inputs', () => {
    expect(gradeOptions([3, 4, 6], [1, 2, 3, 4, 5])).toEqual([3, 4]);
    expect(gradeOptions([], [1, 2])).toEqual([1, 2]);
    expect(excludedIncludes({ includeIds: ['a', 'b'], excludeIds: ['b'] })).toEqual(['b']);
    expect(['max_entries', 'seeding_policy', 'registration_close_at', 'auto_confirm', 'subject_id', 'theme_code'].map(paramInput)).toEqual(['integer', 'seeding_policy', 'datetime', 'boolean', 'subject', 'text']);
  });

  it('lms.challenges translations: vi and en have the same keys, none empty', () => {
    const v = keys((vi as Node).lms && ((vi as Node).lms as Node).challenges);
    const e = keys((en as Node).lms && ((en as Node).lms as Node).challenges);
    expect(v.length).toBeGreaterThan(50);
    expect(e.sort()).toEqual(v.sort());
  });
});
