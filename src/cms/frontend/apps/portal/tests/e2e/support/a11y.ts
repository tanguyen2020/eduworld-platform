// axe-core of the Playwright E2E suites (TC-UI-006, NFR-A11Y-003): same rule set as the Storybook test-runner of
// ops-ui (WCAG 2.0/2.1 A + AA); a violation of impact serious or critical fails. Only rule ids, impacts and CSS
// targets are reported — never the text of the page (student data, AGENTS R14).
import type { Page } from '@playwright/test';
import { createRequire } from 'node:module';

const require = createRequire(import.meta.url);
const AXE_PATH = require.resolve('axe-core/axe.min.js');

export interface AxeFinding {
  id: string;
  impact: string;
  targets: string[];
}

/** Runs axe on `selector` of the page and returns the serious/critical violations. */
export async function axeViolations(page: Page, selector = 'body'): Promise<AxeFinding[]> {
  if (!(await page.evaluate(() => 'axe' in window))) await page.addScriptTag({ path: AXE_PATH });
  return page.evaluate(async (sel) => {
    const axe = (window as unknown as { axe: { run: (ctx: unknown, opts: unknown) => Promise<{ violations: { id: string; impact: string | null; nodes: { target: unknown[] }[] }[] }> } }).axe;
    const r = await axe.run(sel, { runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'] } });
    return r.violations
      .filter((v) => v.impact === 'serious' || v.impact === 'critical')
      .map((v) => ({ id: v.id, impact: v.impact ?? '', targets: v.nodes.slice(0, 5).map((n) => n.target.map(String).join(' ')) }));
  }, selector);
}

/** One line per violation for an assertion message. */
export const describeFindings = (f: AxeFinding[]) => f.map((v) => `${v.impact} ${v.id}: ${v.targets.join(' | ')}`).join('\n');
