// F054 T043–T045 (pure parts), T029/T031 rules, T037 pre-check, T020 response builder, T051 filters:
// question set state ↔ body + blockers (F054-AC-20), lesson blocks (AC-21), blueprint matrix ↔ rules, totals and
// pool by cell (AC-22), review overdue / bulk cap / field notes (AC-12…AC-14), media pre-check (AC-17), try-out
// responses (AC-10), list filters → query (AC-24).
import { describe, expect, it } from 'vitest';
import { cellKey, cellPools, fromRules, ruleIndexOfCells, toRules, totalPoints } from '../blueprints/matrixModel';
import { blueprintIssues, emptyBlueprint, fromBlueprint, toBlueprintBody } from '../blueprints/blueprintModel';
import { addBlock, emptyLesson, fromLesson, lessonIssues, LESSON_LIMITS, moveBlock, toLessonBody } from '../lessons/lessonModel';
import { filtersToQuery } from '../list/useContentList';
import { checkMediaFile, kindOfFile } from '../media/mediaRules';
import { buildResponse, canTryOut, emptyAnswer, moveItem } from '../preview/tryOutResponse';
import { emptyQuestionSet, fromQuestionSet, questionSetIssues, toQuestionSetBody } from '../question-sets/questionSetModel';
import { bulkAllowed, isOverdue, toFieldComments } from '../review/reviewRules';
import type { BlueprintAuthoring, LessonAuthoring, QuestionSetAuthoring } from './useDocument';

const t = (k: string, o?: Record<string, unknown>) => (o ? `${k} ${JSON.stringify(o)}` : k);
const U = (n: number) => `0193a4d2-0000-7000-8000-${String(n).padStart(12, '0')}`;

describe('question set model (F054-AC-20)', () => {
  it('static needs items, rule based needs rules, counts 1–100, difficulty range', () => {
    const s = { ...emptyQuestionSet(), title: { vi: 'Bộ 1' } };
    expect(questionSetIssues(s, t).map((b) => b.field)).toEqual(['items']);
    const rb = { ...s, kind: 'rule_based' as const, rules: [{ filters: { difficulty: { min: 4, max: 2 } }, count: 0 }] };
    expect(questionSetIssues(rb, t).map((b) => b.field)).toEqual(['rules.0.count', 'rules.0.difficulty']);
  });

  it('body keeps unknown definition keys, positions items, drops empty filters, omits items for rule_based', () => {
    const server = { content_type: 'question_set', content_id: U(1), version: 2, status: 'draft', title: { vi: 'Bộ' }, definition: { kind: 'mixed', purpose: 'quest', items: [{ question_version_id: U(5) }], selection_rules: [{ filters: { topic_ids: [U(6)] }, count: 3 }], estimated_duration_sec: 300 } } as unknown as QuestionSetAuthoring;
    const s = fromQuestionSet(server);
    expect(s.kind).toBe('mixed');
    const body = toQuestionSetBody({ ...s, rules: [...s.rules, { filters: { topic_ids: [], difficulty: {} }, count: 2 }] }) as { definition: Record<string, unknown> };
    expect(body.definition.estimated_duration_sec).toBe(300);
    expect(body.definition.items).toEqual([{ question_version_id: U(5), position: 1 }]);
    expect((body.definition.selection_rules as unknown[])[1]).toEqual({ filters: {}, count: 2 });
    expect((toQuestionSetBody({ ...s, kind: 'rule_based' }) as { definition: Record<string, unknown> }).definition.items).toBeUndefined();
  });
});

describe('lesson model (F054-AC-21)', () => {
  it('adds blocks up to maxItems, moves them, and maps worked examples / math_assets', () => {
    let blocks = emptyLesson().blocks;
    blocks = addBlock(blocks, 'worked_example');
    blocks = addBlock(blocks, 'activity');
    expect(blocks.map((b) => b.kind)).toEqual(['rich_text', 'worked_example', 'activity']);
    expect(moveBlock(blocks, 2, 0).map((b) => b.kind)).toEqual(['activity', 'rich_text', 'worked_example']);
    const many = Array.from({ length: LESSON_LIMITS.blocks }, () => blocks[0]!);
    expect(addBlock(many, 'media')).toHaveLength(LESSON_LIMITS.blocks);
    const body = toLessonBody({ ...emptyLesson(), title: { vi: 'Bài' }, blocks }, [{ latex: 'x^2', svg: '<svg/>', speech_text: 'x mũ 2' }]) as { blocks: Record<string, unknown>[]; math_assets: unknown[] };
    expect(body.blocks[1]!.kind).toBe('worked_example');
    expect(body.blocks[1]!.id).toMatch(/^we_[a-z0-9_]{1,40}$/);
    expect((body.blocks[1]!.steps as Record<string, unknown>[])[0]).not.toHaveProperty('audio_ref');
    expect(body.blocks[2]).toEqual({ kind: 'activity', required: true, activity_id: undefined });
    expect(body.math_assets).toHaveLength(1);
  });

  it('blockers: title, media without asset, invalid activity id, required activity for the completion rule', () => {
    const s = { ...emptyLesson(), blocks: addBlock(addBlock([], 'media'), 'activity').map((b) => ({ ...b, required: false })) };
    expect(lessonIssues(s, t).map((b) => b.field)).toEqual(['title', 'blocks.0', 'blocks.1', 'completion_rule']);
    const server = { title: { vi: 'B' }, grade_level: 3, topic_id: U(2), blocks: [{ kind: 'activity', activity_id: U(3), required: true }], completion_rule: 'view_only' } as unknown as LessonAuthoring;
    const back = fromLesson(server);
    expect(back.grade_levels).toEqual([3]);
    expect(lessonIssues(back, t)).toEqual([]);
  });
});

describe('blueprint matrix (F054-AC-22)', () => {
  const rules = [
    { objective_ids: [U(31)], difficulty: { min: 1, max: 1 }, count: 2, points_each: 1 },
    { objective_ids: [U(32)], difficulty: { min: 2, max: 2 }, count: 3, points_each: 2 },
    { topic_ids: [U(20)], count: 1, points_each: 2 },
  ];

  it('rules ↔ matrix keep cells and other rules; totals; rule index of each cell', () => {
    const m = fromRules(rules);
    expect(m.objectives).toEqual([U(31), U(32)]);
    expect(m.difficulties).toEqual([1, 2]);
    expect(m.counts[cellKey(U(32), 2)]).toBe(3);
    expect(m.others).toHaveLength(1);
    expect(toRules(m)).toEqual(rules);
    expect(totalPoints(toRules(m))).toBe(10);
    expect(ruleIndexOfCells(m)).toEqual({ [cellKey(U(31), 1)]: 0, [cellKey(U(32), 2)]: 1 });
  });

  it('pool stats by section + rule index → cells (QUESTION_POOL_INSUFFICIENT highlight)', () => {
    const m = fromRules(rules);
    const pools = cellPools(m, { rules: [{ section_id: 'sec_main', rule_index: 1, required: 3, available: 1, insufficient: true, warning: true }] }, 'sec_main');
    expect(pools[cellKey(U(32), 2)]).toMatchObject({ insufficient: true, available: 1 });
    expect(pools[cellKey(U(31), 1)]).toBeUndefined();
  });

  it('blueprint state ↔ body: first section edited, other sections and keys kept; total points checked', () => {
    const server = { title: { vi: 'Đề' }, subject_id: U(10), grade_levels: [3], exam_type: 'chapter', definition: { duration_sec: 2700, total_points: 10, sections: [{ id: 'sec_a', title: { vi: 'A' }, shuffle_items: false, rules }, { id: 'sec_b', title: { vi: 'B' }, rules: [] }], attempt_policy: { max_attempts: 2, score_policy: 'last' }, result_release: { mode: 'manual' }, navigation: 'free' } } as unknown as BlueprintAuthoring;
    const s = fromBlueprint(server);
    expect(s.duration_min).toBe(45);
    expect(blueprintIssues(s, t)).toEqual([]);
    const body = toBlueprintBody(s) as { definition: { sections: Record<string, unknown>[]; navigation: string; result_release: unknown } };
    expect(body.definition.sections[0]).toMatchObject({ id: 'sec_a', shuffle_items: false, rules });
    expect(body.definition.sections[1]).toMatchObject({ id: 'sec_b' });
    expect(body.definition.navigation).toBe('free');
    expect(body.definition.result_release).toEqual({ mode: 'manual' });
    expect(blueprintIssues({ ...s, total_points: 12 }, t).map((b) => b.field)).toEqual(['matrix']);
    expect(blueprintIssues(emptyBlueprint('Phần 1'), t).map((b) => b.field)).toEqual(['title', 'matrix', 'grade_levels', 'subject_id']);
  });
});

describe('review rules (F054-AC-12, AC-13, AC-14)', () => {
  const now = new Date('2026-11-20T00:00:00Z');
  it('overdue after due_at, else submitted + content_review_sla_days (3)', () => {
    expect(isOverdue({ submitted_at: '2026-11-16T00:00:00Z' }, now)).toBe(true);
    expect(isOverdue({ submitted_at: '2026-11-18T00:00:00Z' }, now)).toBe(false);
    expect(isOverdue({ submitted_at: '2026-11-18T00:00:00Z', due_at: '2026-11-19T00:00:00Z' }, now)).toBe(true);
    expect(isOverdue({ submitted_at: '2026-11-01T00:00:00Z', status: 'approved' }, now)).toBe(false);
  });
  it('bulk cap content_review_bulk_max = 50; field comments as JSON pointers (OQ F054-1)', () => {
    expect(bulkAllowed(50)).toBe(true);
    expect(bulkAllowed(51)).toBe(false);
    expect(bulkAllowed(0)).toBe(false);
    expect(toFieldComments([{ path: ' stem/vi ', comment: ' Thiếu đơn vị ' }, { path: '/answer_key', comment: 'Sai' }, { path: '', comment: 'Cả câu' }, { path: '/hints', comment: '  ' }])).toEqual([
      { path: '/stem/vi', comment: 'Thiếu đơn vị' },
      { path: '/answer_key', comment: 'Sai' },
      { path: '', comment: 'Cả câu' },
    ]);
    expect(toFieldComments([{ path: 'x'.repeat(256), comment: 'c' }])[0]!.path).toHaveLength(256);
  });
});

describe('media pre-check (F054-AC-17)', () => {
  it('kind by extension, type and size limits by kind', () => {
    expect(kindOfFile('a.PNG')).toBe('image');
    expect(checkMediaFile({ name: 'virus.exe', size: 10 })).toEqual({ ok: false, code: 'MEDIA_TYPE_NOT_ALLOWED' });
    expect(checkMediaFile({ name: 'big.png', size: 11 * 1024 * 1024 })).toMatchObject({ ok: false, code: 'MEDIA_TOO_LARGE', kind: 'image' });
    expect(checkMediaFile({ name: 'song.mp3', size: 19 * 1024 * 1024 })).toEqual({ ok: true, kind: 'audio' });
    expect(checkMediaFile({ name: 'renamed.exe.png', size: 100 })).toEqual({ ok: true, kind: 'image' }); // the pipeline rejects it
  });
});

describe('try-out responses (F054-AC-10)', () => {
  it('builds contract responses only when complete', () => {
    expect(canTryOut('matching')).toBe(false);
    expect(buildResponse({ kind: 'single' }, {})).toBeNull();
    expect(buildResponse({ kind: 'single', optionId: 'o_a' }, {})).toEqual({ option_id: 'o_a' });
    const tf = { statements: [{ id: 's_1' }, { id: 's_2' }] };
    expect(buildResponse({ kind: 'true_false', statements: { s_1: true } }, tf)).toBeNull();
    expect(buildResponse({ kind: 'true_false', statements: { s_1: true, s_2: false } }, tf)).toEqual({ statement_values: { s_1: true, s_2: false } });
    expect(emptyAnswer('fill_in_blank', { blanks: [{ id: 'b_1' }] })).toEqual({ kind: 'blanks', blanks: { b_1: '' } });
    expect(buildResponse({ kind: 'blanks', blanks: { b_1: ' 56 ' } }, {})).toEqual({ blanks: { b_1: '56' } });
    expect(buildResponse({ kind: 'ordering', order: ['a', 'b', 'c'] }, {})).toEqual({ order: ['a', 'b', 'c'] });
    expect(moveItem(['a', 'b', 'c'], 0, 1)).toEqual(['b', 'a', 'c']);
  });
});

describe('list filters (F054-AC-24)', () => {
  it('drops empty values; question-only filters ignored for other types', () => {
    const f = { q: ' phan so ', subject_id: U(10), grade_level: 3, status: 'published', type: 'numeric', difficulty: [1, 2], source: 'assigned' as const, topic_id: '', objective_id: undefined };
    expect(filtersToQuery('question', f)).toEqual({ q: 'phan so', subject_id: U(10), grade_level: 3, status: 'published', type: 'numeric', difficulty: [1, 2], source: 'assigned' });
    expect(filtersToQuery('lesson', f)).toEqual({ q: 'phan so', subject_id: U(10), grade_level: 3, status: 'published' });
  });
});
