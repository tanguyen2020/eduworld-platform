// F054 T071 (from F035): blueprint fixed forms — variants.strategy = fixed and every rule of the section lists
// fixed_question_version_ids (efficacy instrument blueprints, LEARNING_EFFICACY_PLAN §3.2). Pure model.
import { describe, expect, it } from 'vitest';
import type { BlueprintAuthoring } from '../builders/useDocument';
import { activeRules, blueprintIssues, emptyBlueprint, fixedRules, fromBlueprint, toBlueprintBody } from './blueprintModel';

const t = (k: string, o?: Record<string, unknown>) => (o ? `${k} ${JSON.stringify(o)}` : k);
const Q = (n: number) => `0193a4d2-0000-7000-8000-${String(n).padStart(12, '0')}`;

describe('fixed form blueprint (F054 T071)', () => {
  it('fixed items → rules grouped by consecutive points (order kept), at most 100 per rule', () => {
    expect(fixedRules([{ question_version_id: Q(1), points: 1 }, { question_version_id: Q(2), points: 1 }, { question_version_id: Q(3), points: 2 }, { question_version_id: Q(4), points: 1 }])).toEqual([
      { fixed_question_version_ids: [Q(1), Q(2)], count: 2, points_each: 1 },
      { fixed_question_version_ids: [Q(3)], count: 1, points_each: 2 },
      { fixed_question_version_ids: [Q(4)], count: 1, points_each: 1 },
    ]);
    const many = Array.from({ length: 101 }, (_, i) => ({ question_version_id: Q(100 + i), points: 0.25 }));
    expect(fixedRules(many).map((r) => r.count)).toEqual([100, 1]);
  });

  it('server fixed form ↔ state ↔ body: variants.strategy fixed, every rule fixed, other definition keys kept', () => {
    const rules = [{ fixed_question_version_ids: [Q(1), Q(2)], count: 2, points_each: 1 }, { fixed_question_version_ids: [Q(3)], count: 1, points_each: 2 }];
    const server = {
      title: { vi: 'Đề đo đầu kỳ' },
      subject_id: Q(10),
      grade_levels: [4],
      exam_type: 'custom',
      efficacy_instrument: true,
      definition: {
        duration_sec: 1800,
        total_points: 4,
        variants: { strategy: 'fixed' },
        navigation: 'free',
        sections: [{ id: 'sec_main', title: { vi: 'P1' }, rules }],
        attempt_policy: { max_attempts: 1, score_policy: 'first' },
        result_release: { mode: 'manual' },
      },
    } as unknown as BlueprintAuthoring;
    const s = fromBlueprint(server);
    expect(s.mode).toBe('fixed');
    expect(s.fixed).toEqual([
      { question_version_id: Q(1), points: 1 },
      { question_version_id: Q(2), points: 1 },
      { question_version_id: Q(3), points: 2 },
    ]);
    expect(blueprintIssues(s, t)).toEqual([]);
    const body = toBlueprintBody(s) as { efficacy_instrument: boolean; definition: { variants: unknown; navigation: string; sections: { rules: unknown[] }[] } };
    expect(body.efficacy_instrument).toBe(true);
    expect(body.definition.variants).toEqual({ strategy: 'fixed' });
    expect(body.definition.navigation).toBe('free');
    expect(body.definition.sections[0]!.rules).toEqual(rules);
  });

  it('switching a new blueprint to a fixed form sends variants.strategy fixed; matrix mode keeps variants as they were', () => {
    const s = { ...emptyBlueprint('Phần 1'), mode: 'fixed' as const, fixed: [{ question_version_id: Q(1), points: 10 }] };
    const body = toBlueprintBody(s) as { definition: { variants?: unknown; sections: { rules: unknown[] }[] } };
    expect(body.definition.variants).toEqual({ strategy: 'fixed' });
    expect(body.definition.sections[0]!.rules).toEqual([{ fixed_question_version_ids: [Q(1)], count: 1, points_each: 10 }]);
    expect(activeRules(s)).toHaveLength(1);
    const matrix = toBlueprintBody(emptyBlueprint('Phần 1')) as { definition: { variants?: unknown } };
    expect(matrix.definition.variants).toBeUndefined();
    const ind = fromBlueprint({ definition: { variants: { strategy: 'independent_draw', count: 2 }, sections: [{ id: 'sec_main', rules: [] }] } } as unknown as BlueprintAuthoring);
    expect(ind.mode).toBe('matrix');
    expect((toBlueprintBody(ind) as { definition: { variants: unknown } }).definition.variants).toEqual({ strategy: 'independent_draw', count: 2 });
  });

  it('issues: no question, duplicate, points mismatch; an efficacy blueprint needs the fixed form', () => {
    const base = { ...emptyBlueprint('Phần 1'), title: { vi: 'Đề' }, subject_id: Q(10), grade_levels: [4], total_points: 3 };
    expect(blueprintIssues({ ...base, mode: 'fixed', fixed: [] }, t).map((b) => b.field)).toEqual(['fixed']);
    const dup = [{ question_version_id: Q(1), points: 1 }, { question_version_id: Q(1), points: 2 }];
    expect(blueprintIssues({ ...base, mode: 'fixed', fixed: dup }, t).map((b) => b.message)).toEqual(['content_studio.blueprint.issue.fixedDuplicate']);
    expect(blueprintIssues({ ...base, mode: 'fixed', fixed: [{ question_version_id: Q(1), points: 1 }] }, t).map((b) => b.field)).toEqual(['fixed']);
    const matrixEfficacy = { ...base, total_points: 10, efficacy_instrument: true };
    expect(blueprintIssues(matrixEfficacy, t).map((b) => b.field)).toContain('efficacy_instrument');
  });
});
