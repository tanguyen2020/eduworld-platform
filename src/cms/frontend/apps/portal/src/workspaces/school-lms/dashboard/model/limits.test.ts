// AGENTS R33 (F056): the dashboard parameters mirrored by the portal equal their registration in CONFIG_DEFAULTS.
import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { DASHBOARD_CONFIG } from './limits';

const REPO = path.resolve(__dirname, '../../../../../../../../../..');
const table = fs.readFileSync(path.join(REPO, 'docs/00-overview/CONFIG_DEFAULTS.md'), 'utf8');

/** First number of the value column of `| \`name\` | value | …`. */
function registered(name: string): number {
  const row = table.split('\n').find((l) => l.startsWith(`| \`${name}\` |`));
  if (!row) throw new Error(`${name} not registered in CONFIG_DEFAULTS`);
  const value = row.split('|')[2]!.replace(/\./g, '');
  return Number(/\d+/.exec(value)![0]);
}

describe('dashboard parameters mirror CONFIG_DEFAULTS', () => {
  for (const [name, value] of Object.entries(DASHBOARD_CONFIG)) {
    it(name, () => expect(registered(name)).toBe(value));
  }
});
