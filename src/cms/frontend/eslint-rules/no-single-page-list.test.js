// Owner rule 2026-11-22 (ADMIN_PORTAL §1.6): single-page list loads and hand-written cursor loops fail lint.
import { RuleTester } from 'eslint';
import tseslint from 'typescript-eslint';
import { describe, it } from 'vitest';
import rule from './no-single-page-list.js';

RuleTester.describe = describe;
RuleTester.it = it;
RuleTester.itOnly = it.only;

const FILE = 'apps/portal/src/workspaces/x/useX.ts';
const tester = new RuleTester({ languageOptions: { parser: tseslint.parser } });

tester.run('ew/no-single-page-list', rule, {
  valid: [
    { code: "api.client.GET('/cms/learning/subjects', { params: { query: { ...page } } });", filename: FILE },
    { code: 'const q = { limit: PAGE_SIZE, cursor };', filename: FILE },
    { code: 'const q = { limit: 50 };', filename: FILE },
    { code: 'for (;;) { const p = await fetchPage(); if (!p.page.next_cursor) break; }', filename: 'apps/portal/src/shared/api/fetchAllPages.ts' },
    { code: 'while (i < 3) i++;', filename: FILE },
  ],
  invalid: [
    { code: "api.client.GET('/cms/learning/subjects', { params: { query: { limit: 100 } } });", filename: FILE, errors: [{ messageId: 'bigLimit' }] },
    { code: 'const q = { limit: 200 };', filename: FILE, errors: [{ messageId: 'bigLimit' }] },
    { code: "const q = { 'limit': 500 };", filename: FILE, errors: [{ messageId: 'bigLimit' }] },
    { code: 'do { r = await next(c); c = r.page.next_cursor; } while (c);', filename: FILE, errors: [{ messageId: 'cursorLoop' }] },
  ],
});
