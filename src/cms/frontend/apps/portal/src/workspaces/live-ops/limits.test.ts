// AGENTS R33, ADR-0041 (F080): the dashboard parameters mirrored by workspace E equal their registration in
// CONFIG_DEFAULTS, and the field limits equal the contract (specs/openapi/eduworld-api.v1.yaml).
import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { locate } from '../../test/openapiLocate';
import { DASHBOARD_CONFIG, FIELDS } from './limits';

const REPO = path.resolve(__dirname, '../../../../../../../..');
const table = fs.readFileSync(path.join(REPO, 'docs/00-overview/CONFIG_DEFAULTS.md'), 'utf8');

function registered(name: string): number {
  const row = table.split('\n').find((l) => l.startsWith(`| \`${name}\` |`));
  if (!row) throw new Error(`${name} not registered in CONFIG_DEFAULTS`);
  return Number(/\d+/.exec(row.split('|')[2]!.replace(/\./g, ''))![0]);
}

describe('F080 limits mirror their owners', () => {
  for (const [name, value] of Object.entries(DASHBOARD_CONFIG)) it(name, () => expect(registered(name)).toBe(value));

  it('EventRunOperation.reason', () => {
    const s = locate(FIELDS.runReason.source);
    expect([s.minLength, s.maxLength]).toEqual([FIELDS.runReason.minLength, FIELDS.runReason.maxLength]);
  });

  it('getGamePerformance client_versions', () => {
    const s = locate(FIELDS.clientVersion.source);
    expect(s.maxItems).toBe(FIELDS.clientVersion.maxItems);
    expect((s.items as { maxLength: number }).maxLength).toBe(FIELDS.clientVersion.maxLength);
  });
});
