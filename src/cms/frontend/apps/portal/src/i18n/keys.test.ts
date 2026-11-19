// F050-AC-04 (AGENTS §7): vi and en have the same keys (portal and ops-ui namespaces); JSX literal strings fail
// lint; every ProblemCode resolves to a message (own key or error.generic) — missing own keys are reported.
import { opsUiResources } from '@eduworld/ops-ui';
import { ESLint } from 'eslint';
import fs from 'node:fs';
import yaml from 'js-yaml';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import en from './en.json';
import vi from './vi.json';
import { createI18n } from './index';

function keys(obj: unknown, prefix = ''): string[] {
  if (!obj || typeof obj !== 'object') return [prefix];
  return Object.entries(obj as Record<string, unknown>).flatMap(([k, v]) => keys(v, prefix ? `${prefix}.${k}` : k));
}

const ROOT = path.resolve(__dirname, '../../../..');
const REPO = path.resolve(ROOT, '../../..');

describe('translation files', () => {
  it('portal: vi ⊆ en and en ⊆ vi', () => {
    expect(keys(en).sort()).toEqual(keys(vi).sort());
  });

  it('ops-ui: vi ⊆ en and en ⊆ vi', () => {
    expect(keys(opsUiResources.en).sort()).toEqual(keys(opsUiResources.vi).sort());
  });

  it('no empty translation', () => {
    const empty = (o: unknown, p: string): string[] => keys(o).filter((k) => {
      const v = k.split('.').reduce<unknown>((n, s) => (n as Record<string, unknown>)?.[s], o);
      return typeof v !== 'string' || v.trim() === '';
    }).map((k) => `${p}:${k}`);
    expect([...empty(vi, 'vi'), ...empty(en, 'en')]).toEqual([]);
  });

  it('every ProblemCode of the spec displays a message (own key or error.generic)', async () => {
    const spec = yaml.load(fs.readFileSync(path.join(REPO, 'specs/openapi/eduworld-api.v1.yaml'), 'utf8')) as {
      components: { schemas: { ProblemCode: { enum: string[] } } };
    };
    const codes = spec.components.schemas.ProblemCode.enum;
    const i18n = await createI18n('vi-VN');
    const missing = codes.filter((c) => !i18n.exists(`error.${c}`));
    for (const c of codes) {
      const msg = i18n.t(i18n.exists(`error.${c}`) ? `error.${c}` : 'error.generic');
      expect(msg.length, c).toBeGreaterThan(0);
    }
    // warning only (spec edge case): new codes fall back to error.generic until a message is written
    if (missing.length) console.warn(`[i18n] ${missing.length}/${codes.length} ProblemCode without own message (error.generic is shown)`);
    expect(missing.length).toBeLessThan(codes.length);
  });
});

describe('lint: UI strings come from translation files', () => {
  const eslint = new ESLint({ cwd: ROOT });
  const lint = async (code: string) =>
    (await eslint.lintText(code, { filePath: path.join(ROOT, 'apps/portal/src/app/Probe.tsx') }))[0]!.messages.map((m) => m.ruleId);

  it('JSX text literal fails', async () => {
    expect(await lint('export const Probe = () => <p>Xin chào</p>;\n')).toContain('i18next/no-literal-string');
  });

  it('visible attribute literal fails', async () => {
    expect(await lint('export const Probe = () => <input placeholder="Tìm kiếm" />;\n')).toContain('i18next/no-literal-string');
  });

  it('translated text passes', async () => {
    const code = 'import { useTranslation } from "react-i18next";\nexport const Probe = () => { const { t } = useTranslation(); return <p>{t("app.title")}</p>; };\n';
    expect(await lint(code)).not.toContain('i18next/no-literal-string');
  });
});
