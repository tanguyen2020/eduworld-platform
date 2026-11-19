// @vitest-environment node
// F050-AC-02: manifest check — one dynamic chunk per workspace, none in the entry's static import closure.
import { describe, expect, it } from 'vitest';
import { checkManifest, WORKSPACES } from './check-chunks.mjs';

const good = () => {
  const m = { 'index.html': { file: 'assets/index-a.js', isEntry: true, imports: ['_vendor.js'], dynamicImports: [] } };
  m['_vendor.js'] = { file: 'assets/vendor-b.js' };
  for (const w of WORKSPACES) {
    const key = `src/workspaces/${w}/index.tsx`;
    m['index.html'].dynamicImports.push(key);
    m[key] = { file: `assets/workspace-${w}-x.js`, isDynamicEntry: true, imports: ['_vendor.js'] };
  }
  return m;
};

describe('checkManifest', () => {
  it('accepts 5 lazy workspace chunks', () => {
    expect(checkManifest(good())).toEqual([]);
  });

  it('rejects a workspace imported statically by the entry', () => {
    const m = good();
    m['index.html'].imports.push('src/workspaces/live-ops/index.tsx');
    expect(checkManifest(m)).toEqual(['live-ops: statically imported by the entry chunk (loaded on /)']);
  });

  it('rejects a missing workspace chunk and a non-dynamic one', () => {
    const m = good();
    delete m['src/workspaces/game-ops/index.tsx'];
    m['src/workspaces/school-lms/index.tsx'].isDynamicEntry = false;
    expect(checkManifest(m)).toEqual(expect.arrayContaining(['game-ops: no chunk for src/workspaces/game-ops/index.tsx', 'school-lms: chunk is not a dynamic entry']));
  });
});
