// blueprintMatrix (F054-AC-22, specs/schemas/assessment-blueprint.schema.json, EXAM_ENGINE §3 — pure): the
// "LO × độ khó" matrix of one section ↔ its rules (one rule per non-empty cell: objective_ids = [LO],
// difficulty = {min: d, max: d}, count, points_each of the column), total points check, and the cells whose pool is
// insufficient (getAssessmentBlueprintPoolStats `insufficient` / `warning` by section + rule index). Rules that are not
// a single LO × single difficulty are kept as "other rules" untouched.
import type { PoolStats } from '../builders/useDocument';

export interface BpRule {
  objective_ids?: string[];
  topic_ids?: string[];
  difficulty?: { min?: number; max?: number };
  types?: string[];
  fixed_question_version_ids?: string[];
  count: number;
  points_each: number;
  [k: string]: unknown;
}

export interface Matrix {
  objectives: string[];
  difficulties: number[];
  /** `${lo}:${d}` → count */
  counts: Record<string, number>;
  /** difficulty → points of one question */
  points: Record<number, number>;
  /** Rules that are not matrix cells (kept). */
  others: BpRule[];
}

export const cellKey = (lo: string, d: number) => `${lo}:${d}`;
export const DEFAULT_DIFFICULTIES = [1, 2, 3];
export const BP_LIMITS = { rules: 20, count: 100 } as const;

const isCell = (r: BpRule) => r.objective_ids?.length === 1 && !r.topic_ids?.length && !r.fixed_question_version_ids?.length && !r.types?.length && r.difficulty?.min !== undefined && r.difficulty.min === r.difficulty.max;

export function fromRules(rules: readonly BpRule[], objectives: readonly string[] = []): Matrix {
  const m: Matrix = { objectives: [...objectives], difficulties: [], counts: {}, points: {}, others: [] };
  for (const r of rules) {
    if (!isCell(r)) {
      m.others.push(r);
      continue;
    }
    const lo = r.objective_ids![0]!;
    const d = r.difficulty!.min!;
    if (!m.objectives.includes(lo)) m.objectives.push(lo);
    if (!m.difficulties.includes(d)) m.difficulties.push(d);
    m.counts[cellKey(lo, d)] = (m.counts[cellKey(lo, d)] ?? 0) + r.count;
    m.points[d] = r.points_each;
  }
  if (m.difficulties.length === 0) m.difficulties = [...DEFAULT_DIFFICULTIES];
  m.difficulties.sort((a, b) => a - b);
  for (const d of m.difficulties) m.points[d] ??= 1;
  return m;
}

/** Rule order: LO rows top to bottom, difficulty left to right, then the other rules. */
export function toRules(m: Matrix): BpRule[] {
  const out: BpRule[] = [];
  for (const lo of m.objectives)
    for (const d of m.difficulties) {
      const count = m.counts[cellKey(lo, d)] ?? 0;
      if (count > 0) out.push({ objective_ids: [lo], difficulty: { min: d, max: d }, count, points_each: m.points[d] ?? 1 });
    }
  return [...out, ...m.others];
}

/** Rule index (in toRules order) of each non-empty cell. */
export function ruleIndexOfCells(m: Matrix): Record<string, number> {
  const idx: Record<string, number> = {};
  let i = 0;
  for (const lo of m.objectives)
    for (const d of m.difficulties) {
      if ((m.counts[cellKey(lo, d)] ?? 0) > 0) idx[cellKey(lo, d)] = i++;
    }
  return idx;
}

export function totalPoints(rules: readonly BpRule[]): number {
  return Math.round(rules.reduce((n, r) => n + r.count * r.points_each, 0) * 100) / 100;
}

export function totalQuestions(rules: readonly BpRule[]): number {
  return rules.reduce((n, r) => n + r.count, 0);
}

export type CellPool = { available: number; required: number; insufficient: boolean; warning: boolean };

/** Pool of each cell of section `sectionId` (rule_index of the pool stats = index in the section's rules). */
export function cellPools(m: Matrix, stats: PoolStats | undefined, sectionId: string): Record<string, CellPool> {
  if (!stats) return {};
  const idx = ruleIndexOfCells(m);
  const out: Record<string, CellPool> = {};
  for (const [cell, i] of Object.entries(idx)) {
    const r = stats.rules.find((x) => x.rule_index === i && (!x.section_id || x.section_id === sectionId));
    if (r) out[cell] = { available: r.available, required: r.required, insufficient: r.insufficient, warning: r.warning };
  }
  return out;
}
