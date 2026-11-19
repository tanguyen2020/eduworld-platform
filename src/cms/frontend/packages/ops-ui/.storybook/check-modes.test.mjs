// @vitest-environment node
// F050-AC-18: the mode-coverage check of ops-ui stories.
import { describe, expect, it } from 'vitest';
import { checkTree, readTree } from './check-modes.mjs';

describe('check-modes', () => {
  it('the repository ops-ui passes', () => {
    expect(checkTree(readTree())).toEqual([]);
  });

  it('a component without stories fails', () => {
    expect(checkTree({ 'components/X/X.tsx': 'export function X() { return null; }' })).toEqual([
      'components/X/X.tsx: component X has no components/X/X.stories.tsx',
    ]);
  });

  it('a stories file without the Dark story fails', () => {
    const files = {
      'components/X/X.tsx': 'export function X() { return null; }',
      'components/X/X.stories.tsx': 'export const Light: Story = {};\nexport const Compact: Story = {};\n',
    };
    expect(checkTree(files)).toEqual(['components/X/X.stories.tsx: missing stories Dark']);
  });
});
