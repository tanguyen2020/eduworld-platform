// F052 T002 (AGENTS §7, F050-AC-04): the workspace C translation files have the same keys in vi and en, no empty
// string, and every `portal.school_lms.*` key used in the code exists (dynamic keys checked by their prefix).
import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import en from '../../i18n/locales/en/portal.school_lms.json';
import vi from '../../i18n/locales/vi/portal.school_lms.json';

function leaves(obj: unknown, prefix = ''): [string, unknown][] {
  if (!obj || typeof obj !== 'object') return [[prefix, obj]];
  const entries = Object.entries(obj as Record<string, unknown>);
  return entries.length ? entries.flatMap(([k, v]) => leaves(v, prefix ? `${prefix}.${k}` : k)) : [];
}

const files = (dir: string): string[] =>
  fs.readdirSync(dir, { withFileTypes: true }).flatMap((e) => (e.isDirectory() ? files(path.join(dir, e.name)) : /\.tsx?$/.test(e.name) && !/\.test\./.test(e.name) ? [path.join(dir, e.name)] : []));

describe('portal.school_lms translations', () => {
  it('vi and en have the same keys, none empty', () => {
    const v = leaves(vi);
    const e = leaves(en);
    expect(e.map(([k]) => k).sort()).toEqual(v.map(([k]) => k).sort());
    expect([...v, ...e].filter(([, x]) => typeof x !== 'string' || x.trim() === '')).toEqual([]);
  });

  it('every key used by the workspace exists', () => {
    const known = new Set(leaves(vi).map(([k]) => `portal.school_lms.${k}`));
    const missing: string[] = [];
    for (const f of files(__dirname)) {
      const src = fs.readFileSync(f, 'utf8');
      for (const m of src.matchAll(/['`](portal\.school_lms\.[A-Za-z0-9_.]+)(\$\{)?/g)) {
        const key = m[1]!;
        const dynamic = !!m[2] || key.endsWith('.');
        if (dynamic ? ![...known].some((k) => k.startsWith(key.endsWith('.') ? key : `${key}.`) || k.startsWith(key)) : !known.has(key)) missing.push(`${path.basename(f)}: ${key}`);
      }
    }
    expect(missing).toEqual([]);
  });
});
