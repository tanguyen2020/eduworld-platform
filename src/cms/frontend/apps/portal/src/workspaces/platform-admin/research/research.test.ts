// @vitest-environment node
// F035 T066 (ADR-0041, AGENTS R28/R33, §7): every input limit of the research screens equals the schema field it
// names in specs/openapi/eduworld-api.v1.yaml; the rule_params offered are the backend catalog; vi/en translations
// have the same keys and none is empty; every key used by the screens exists.
import fs from 'node:fs';
import path from 'node:path';
import yaml from 'js-yaml';
import { describe, expect, it } from 'vitest';
import { EXPERIMENT_KINDS, RULE_PARAMS, STUDY_STATUSES } from './api';
import en from './i18n/en.json';
import vi from './i18n/vi.json';
import { RESEARCH_LIMITS } from './limits';

const REPO = path.resolve(__dirname, '../../../../../../../../..');
type Node = Record<string, unknown>;
const spec = yaml.load(fs.readFileSync(path.join(REPO, 'specs/openapi/eduworld-api.v1.yaml'), 'utf8')) as Node;
const schemas = (spec.components as Node).schemas as Node;
const deref = (n: Node): Node => (typeof n.$ref === 'string' ? deref(n.$ref.slice(2).split('/').reduce<Node>((o, k) => o[k] as Node, spec)) : n);

describe('limits of the research screens = OpenAPI', () => {
  it.each(Object.entries(RESEARCH_LIMITS))('%s', (_name, limit) => {
    const [, , schema, , field] = limit.source.split('.');
    const s = deref(((deref(schemas[schema!] as Node).properties as Node)[field!]) as Node);
    for (const k of ['minLength', 'maxLength', 'pattern', 'minimum', 'maximum', 'maxItems'] as const) {
      if (k in limit) expect(s[k], `${limit.source} ${k}`).toEqual((limit as Record<string, unknown>)[k]);
    }
  });

  it('enums offered by the screens are the OpenAPI enums', () => {
    expect([...STUDY_STATUSES].sort()).toEqual([...((schemas.EfficacyStudyStatus as Node).enum as string[])].sort());
    expect([...EXPERIMENT_KINDS].sort()).toEqual([...((schemas.LearningExperimentKind as Node).enum as string[])].sort());
  });

  it('rule_params offered = the catalog of the backend domain (recommendation.Params)', () => {
    const go = fs.readFileSync(path.join(REPO, 'src/backend/modules/learning/internal/domain/recommendation/params.go'), 'utf8');
    const tags = [...go.matchAll(/json:"([a-z_]+),omitempty"/g)].map((m) => m[1]);
    expect([...RULE_PARAMS].sort()).toEqual(tags.sort());
  });
});

describe('translations of the research screens', () => {
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

  it('every static key of the screens exists; every rule parameter and status has a label', () => {
    const dir = __dirname;
    const used = new Set<string>();
    for (const f of fs.readdirSync(dir).filter((x) => x.endsWith('.tsx'))) {
      for (const m of fs.readFileSync(path.join(dir, f), 'utf8').matchAll(/'platform\.research\.([A-Za-z0-9_.]+)'/g)) used.add(m[1]!);
    }
    expect(used.size).toBeGreaterThan(50);
    for (const k of used) expect(value(vi, k), k).toBeTruthy();
    for (const p of RULE_PARAMS) expect(value(vi, `ruleParam.${p}`), p).toBeTruthy();
    for (const s of [...STUDY_STATUSES, 'shadow', 'live', 'stopped', 'queued', 'running', 'succeeded', 'failed']) expect(value(vi, `status.${s}`), s).toBeTruthy();
  });
});
