// F050-AC-18 (TC-UI-005): every exported React component of ops-ui has a `<Name>.stories.tsx` next to it with
// the stories `Light`, `Dark` and `Compact`. Run before `storybook build` (exit 1 on a gap).
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

export const MODES = ['Light', 'Dark', 'Compact'];
const SRC = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', 'src');

/** Exported PascalCase function components of a TSX source. */
export function exportedComponents(source) {
  return [...source.matchAll(/^export function ([A-Z][A-Za-z0-9]*)\s*\(/gm)].map((m) => m[1]);
}

export function exportedStories(source) {
  return [...source.matchAll(/^export const ([A-Z][A-Za-z0-9]*)\s*(?::[^=]+)?=/gm)].map((m) => m[1]);
}

/** Problems for a tree: {relPath → content}. */
export function checkTree(files) {
  const problems = [];
  for (const [rel, content] of Object.entries(files)) {
    if (!rel.endsWith('.tsx') || /\.(test|stories)\.tsx$/.test(rel) || rel.includes('/test/')) continue;
    for (const name of exportedComponents(content)) {
      const storyRel = path.posix.join(path.posix.dirname(rel), `${name}.stories.tsx`);
      const story = files[storyRel];
      if (!story) {
        problems.push(`${rel}: component ${name} has no ${storyRel}`);
        continue;
      }
      const missing = MODES.filter((m) => !exportedStories(story).includes(m));
      if (missing.length) problems.push(`${storyRel}: missing stories ${missing.join(', ')}`);
    }
  }
  return problems;
}

export function readTree(dir = SRC) {
  const out = {};
  const walk = (d) => {
    for (const e of fs.readdirSync(d, { withFileTypes: true })) {
      const p = path.join(d, e.name);
      if (e.isDirectory()) walk(p);
      else if (e.name.endsWith('.tsx')) out[path.relative(dir, p).split(path.sep).join('/')] = fs.readFileSync(p, 'utf8');
    }
  };
  walk(dir);
  return out;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const tree = readTree();
  const problems = checkTree(tree);
  if (problems.length) {
    console.error(`check-modes: ${problems.length} problem(s)\n  ${problems.join('\n  ')}`);
    process.exit(1);
  }
  const n = Object.entries(tree).filter(([r]) => !/\.(test|stories)\.tsx$/.test(r) && !r.includes('/test/')).flatMap(([, c]) => exportedComponents(c)).length;
  console.log(`check-modes: ${n} component(s), each with Light/Dark/Compact stories`);
}
