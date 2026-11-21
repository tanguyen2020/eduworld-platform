// F055 T006, T011 (F055-AC-03, AC-04, AC-12; ADR-0041): limits and enums of the LMS forms mirror the OpenAPI spec;
// graded mode never sends follow_latest nor default release fields; form checks (maxLength, maxItems, due > open,
// ranges); fields editable per status; server field pointers → wizard fields and steps; ProblemCode → message key.
import fs from 'node:fs';
import path from 'node:path';
import yaml from 'js-yaml';
import { describe, expect, it } from 'vitest';
import { ApiError } from '../../../../shared/api/middleware/problem';
import en from '../../../../i18n/lms.en.json';
import vi from '../../../../i18n/lms.vi.json';
import { EMPTY_FORM, editableFields, excludedIncludes, formFieldOf, stepOf, toCreatePayload, toTargets, toUpdatePayload, validateAssignment, type Assignment, type AssignmentFormValues } from './assignmentForm';
import { LMS_ENUMS, LMS_FIELDS, LMS_LIMITS } from './fieldSchemas';
import { isContentError, isStale, lmsErrorKey, LMS_OWN_CODES } from './problemMessages';

type Node = Record<string, unknown>;
const REPO = path.resolve(__dirname, '../../../../../../../../../..');
const spec = yaml.load(fs.readFileSync(path.join(REPO, 'specs/openapi/eduworld-api.v1.yaml'), 'utf8')) as Node;

function deref(n: Node): Node {
  let cur = n;
  while (typeof cur.$ref === 'string') cur = (cur.$ref as string).slice(2).split('/').reduce<Node>((o, k) => o[k] as Node, spec);
  if (Array.isArray(cur.allOf)) return { ...cur, properties: Object.assign({}, ...(cur.allOf as Node[]).map((s) => deref(s).properties)) };
  return cur;
}

/** `schemas/A/properties/b/properties/c` (nested, allOf merged). */
function at(source: string): Node {
  const [, name, ...rest] = source.split('/');
  let cur = deref(((spec.components as Node).schemas as Node)[name!] as Node);
  for (let i = 0; i < rest.length; i += 2) cur = deref((cur[rest[i]!] as Node)[rest[i + 1]!] as Node);
  return cur;
}

describe('fieldSchemas mirror the OpenAPI spec (ADR-0041, F055-AC-04)', () => {
  for (const [name, f] of Object.entries(LMS_FIELDS)) {
    it(`field ${name}`, () => {
      const s = at(f.source);
      expect(s.maxLength, name).toBe(f.maxLength);
      expect(s.minLength, name).toBe('minLength' in f ? f.minLength : undefined);
    });
  }
  for (const [name, l] of Object.entries(LMS_LIMITS)) {
    it(`limit ${name}`, () => {
      const s = at(l.source);
      for (const k of ['minimum', 'maximum', 'minItems', 'maxItems', 'minLength', 'maxLength'] as const) if (k in l) expect(s[k], `${name}.${k}`).toBe((l as Record<string, unknown>)[k]);
    });
  }
  for (const [name, e] of Object.entries(LMS_ENUMS)) {
    it(`enum ${name}`, () => expect(at(e.source).enum).toEqual(e.values));
  }
});

const filled: AssignmentFormValues = {
  ...EMPTY_FORM,
  content: { contentType: 'activity', contentId: 'c1', versionId: 'v1', title: 'Quiz' },
  schoolId: 'school-a',
  subjectId: 'math',
  grades: [3],
  title: 'Ôn tập',
  openAt: '2026-11-20T01:00:00.000Z',
  dueAt: '2026-11-27T10:00:00.000Z',
};

describe('assignment form (F055-AC-03, AC-04)', () => {
  it('a valid form has no issue; payload has school, subject, a grade target, the pinned version and no release fields', () => {
    expect(validateAssignment(filled)).toEqual([]);
    const p = toCreatePayload(filled);
    expect(p).toMatchObject({ school_id: 'school-a', subject_id: 'math' });
    expect(p).not.toHaveProperty('course_id');
    expect(p.content_ref).toEqual({ content_type: 'activity', content_id: 'c1', pinned_version_id: 'v1' });
    expect(p.targets).toEqual([{ target_type: 'grade', grade_level: 3 }]);
    expect(p).not.toHaveProperty('feedback_timing');
    expect(p).not.toHaveProperty('result_release');
  });

  it('grades: one target per grade (sorted, no duplicate), include on the first, exclude on every grade target (ADR-0095)', () => {
    const v = { ...filled, grades: [4, 3, 4], includeIds: ['s9', 's9'], excludeIds: ['s1', 's9'] };
    expect(toTargets(v)).toEqual([
      { target_type: 'grade', grade_level: 3, include_student_ids: ['s9'], exclude_student_ids: ['s1', 's9'] },
      { target_type: 'grade', grade_level: 4, exclude_student_ids: ['s1', 's9'] },
    ]);
    expect(excludedIncludes(v)).toEqual(['s9', 's9']);
    expect(excludedIncludes(filled)).toEqual([]);
  });

  it('students mode: one students target only; grades, include and exclude are not sent', () => {
    const p = toCreatePayload({ ...filled, targetMode: 'students', studentIds: ['s1', 's2', 's1'], includeIds: ['x'], excludeIds: ['y'] });
    expect(p.targets).toEqual([{ target_type: 'students', student_ids: ['s1', 's2'] }]);
  });

  it('graded: follow_latest is never sent true and the version stays pinned (AC-03)', () => {
    const p = toCreatePayload({ ...filled, mode: 'graded', followLatest: true });
    expect(p.follow_latest).toBe(false);
    expect(p.content_ref.pinned_version_id).toBe('v1');
    expect(p).not.toHaveProperty('result_release');
    const practice = toCreatePayload({ ...filled, followLatest: true });
    expect(practice.follow_latest).toBe(true);
    expect(practice.content_ref).not.toHaveProperty('pinned_version_id');
  });

  it('release fields are sent only when chosen; students target is one target with student_ids', () => {
    const p = toCreatePayload({ ...filled, resultRelease: 'manual', targetMode: 'students', studentIds: ['s1', 's2'] });
    expect(p.result_release).toBe('manual');
    expect(p.targets).toEqual([{ target_type: 'students', student_ids: ['s1', 's2'] }]);
  });

  it('limits: title maxLength, instructions maxLength, grade targets maxItems (9), include/exclude maxItems, due > open, ranges (AC-04)', () => {
    const many = (n: number, p: string) => Array.from({ length: n }, (_, i) => `${p}${i}`);
    const issues = validateAssignment({
      ...filled,
      title: 'x'.repeat(LMS_FIELDS.assignmentTitle.maxLength + 1),
      instructions: 'y'.repeat(LMS_FIELDS.assignmentInstructions.maxLength + 1),
      grades: Array.from({ length: LMS_LIMITS.targets.maxItems + 1 }, (_, i) => i + 1),
      includeIds: many(LMS_LIMITS.includeStudents.maxItems + 1, 'i'),
      excludeIds: many(LMS_LIMITS.excludeStudents.maxItems + 1, 'e'),
      dueAt: '2026-11-19T00:00:00.000Z',
      lateWindowHours: LMS_LIMITS.lateWindowHours.maximum + 1,
      maxAttempts: 0,
    });
    expect(issues.map((i) => `${i.field}:${i.key}`).sort()).toEqual([
      'dueAt:dueAfterOpen',
      'excludeIds:tooMany',
      'grades:tooMany',
      'includeIds:tooMany',
      'instructions:tooLong',
      'lateWindowHours:range',
      'maxAttempts:range',
      'title:tooLong',
    ]);
    expect(validateAssignment({ ...filled, grades: [0] }).map((i) => `${i.field}:${i.key}`)).toEqual(['grades:range']);
    expect(validateAssignment({ ...filled, targetMode: 'students', studentIds: many(LMS_LIMITS.targetStudents.maxItems + 1, 's') }).map((i) => `${i.field}:${i.key}`)).toEqual(['studentIds:tooMany']);
    expect(validateAssignment({ ...EMPTY_FORM }).map((i) => i.field)).toEqual(expect.arrayContaining(['title', 'content', 'schoolId', 'subjectId', 'grades', 'openAt', 'dueAt']));
    expect(validateAssignment({ ...EMPTY_FORM, targetMode: 'students' }).map((i) => i.field)).toEqual(expect.arrayContaining(['studentIds']));
  });

  it('server pointers map to fields and steps', () => {
    expect(formFieldOf('/due_at')).toBe('dueAt');
    expect(formFieldOf('/school_id')).toBe('schoolId');
    expect(formFieldOf('/subject_id')).toBe('subjectId');
    expect(formFieldOf('/targets/0/include_student_ids/3')).toBe('includeIds');
    expect(formFieldOf('/targets/1/exclude_student_ids')).toBe('excludeIds');
    expect(formFieldOf('/targets/0/student_ids/0')).toBe('studentIds');
    expect(formFieldOf('/targets/2/grade_level')).toBe('grades');
    expect(formFieldOf('/targets')).toBe('grades');
    expect(formFieldOf('late_policy.late_window_hours')).toBe('lateWindowHours');
    expect(stepOf('content')).toBe(0);
    expect(stepOf('includeIds')).toBe(1);
    expect(stepOf('subjectId')).toBe(1);
    expect(stepOf('title')).toBe(2);
    expect(stepOf('resultRelease')).toBe(3);
  });
});

describe('edit rules (F055-AC-12)', () => {
  const a = { status: 'open', title: 'A', due_at: '2026-11-27T10:00:00Z', instructions: 'x' } as Assignment;
  it('open → due_at, instructions, late_policy only; closed → nothing', () => {
    expect(editableFields('open').sort()).toEqual(['due_at', 'instructions', 'late_policy']);
    expect(editableFields('draft')).toContain('title');
    expect(editableFields('closed')).toEqual([]);
  });
  it('patch has only changed, allowed fields', () => {
    expect(toUpdatePayload(a, { title: 'B', due_at: '2026-11-27T10:00:00.000Z', instructions: 'y' })).toEqual({ instructions: 'y' });
  });
});

describe('problem messages (F055-AC-06)', () => {
  const err = (status: number, code: string) => new ApiError({ status, code, i18nKey: `error.${code}` });
  it('own LMS messages by code, then the shared error key; 412 = version mismatch', () => {
    expect(lmsErrorKey(err(422, 'CONTENT_NOT_PUBLISHED'))).toBe('lms.errors.CONTENT_NOT_PUBLISHED');
    expect(lmsErrorKey(err(412, 'PRECONDITION_FAILED'))).toBe('lms.errors.VERSION_MISMATCH');
    expect(lmsErrorKey(err(500, 'INTERNAL'))).toBe('error.INTERNAL');
    expect(lmsErrorKey(new Error('x'))).toBe('error.generic');
    expect(isContentError(err(403, 'RESOURCE_NOT_ASSIGNED'))).toBe(true);
    expect(isStale(err(422, 'ASSIGNMENT_NOT_EDITABLE'))).toBe(true);
  });
  it('every own code has a vi and en message; vi and en have the same keys', () => {
    const keys = (o: unknown, p = ''): string[] => (o && typeof o === 'object' ? Object.entries(o).flatMap(([k, v]) => keys(v, p ? `${p}.${k}` : k)) : [p]);
    expect(keys(en).sort()).toEqual(keys(vi).sort());
    for (const c of LMS_OWN_CODES) expect(vi.lms.errors[c], c).toBeTruthy();
  });
});
