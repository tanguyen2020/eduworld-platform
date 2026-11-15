// Kiểm tra cú pháp mọi khối ```mermaid trong docs/ và README.md bằng mermaid.parse (mermaid 11 + jsdom).
// Chạy: node scripts/docs/check-mermaid.mjs [repoRoot]  → exit 1 nếu có sơ đồ lỗi.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { JSDOM } from 'jsdom';

const repoRoot = path.resolve(process.argv[2] ?? path.join(path.dirname(fileURLToPath(import.meta.url)), '..', '..'));
const dom = new JSDOM('<!doctype html><html><body></body></html>', { pretendToBeVisual: true });
globalThis.window = dom.window;
globalThis.document = dom.window.document;
for (const k of ['Element', 'HTMLElement', 'SVGElement', 'DOMParser', 'Node', 'navigator']) {
  try { if (!(k in globalThis)) globalThis[k] = dom.window[k]; } catch { /* ignore */ }
}
const { default: mermaid } = await import('mermaid');
mermaid.initialize({ startOnLoad: false });

const SKIP = new Set(['node_modules', '.git', '.claude']);
const files = [];
function walk(dir) {
  for (const name of fs.readdirSync(dir)) {
    const p = path.join(dir, name);
    const st = fs.statSync(p);
    if (st.isDirectory()) { if (!SKIP.has(name)) walk(p); }
    else if (name.endsWith('.md')) files.push(p);
  }
}
walk(path.join(repoRoot, 'docs'));
for (const f of ['README.md']) { const p = path.join(repoRoot, f); if (fs.existsSync(p)) files.push(p); }

let total = 0;
const bad = [];
for (const file of files) {
  const text = fs.readFileSync(file, 'utf8');
  const re = /```mermaid\r?\n([\s\S]*?)```/g;
  let m;
  while ((m = re.exec(text))) {
    total++;
    const startLine = text.slice(0, m.index).split('\n').length;
    try { await mermaid.parse(m[1]); }
    catch (e) {
      const msg = String(e?.message ?? e);
      const ln = Number(msg.match(/line (\d+)/)?.[1] ?? 0);
      const src = ln ? (m[1].split('\n')[ln - 1] ?? '').trim() : '';
      bad.push(`${path.relative(repoRoot, file)}:${startLine + ln}  ${msg.split('\n')[0]}${src ? `\n    >> ${src}` : ''}`);
    }
  }
}
console.log(`check-mermaid: ${files.length} file, ${total} sơ đồ, ${bad.length} lỗi`);
if (bad.length) { console.log(bad.join('\n')); process.exit(1); }
