// Xóa các file chỉ còn marker "<!-- MERGED-INTO: docs/... -->" và chuyển mọi link trỏ tới chúng sang file đích.
// Anchor được giữ nếu file đích có heading cùng slug; nếu không thì bỏ anchor.
import fs from 'node:fs';
import path from 'node:path';
import { ROOT, walk, rel, abs } from './lib-links.mjs';

const slug = (h) => h.trim().toLowerCase().replace(/[^\p{L}\p{N}\s_-]/gu, '').replace(/\s/g, '-');
const MARK = /^<!--\s*MERGED-INTO:\s*(docs\/[^\s]+?\.md)\s*-->\s*$/;

const files = walk(ROOT).map(rel);
const merged = {};
for (const f of files) {
  const t = fs.readFileSync(abs(f), 'utf8').trim();
  const m = t.match(MARK);
  if (m) merged[f] = m[1];
}
// giải chuỗi gộp (A -> B -> C)
const finalOf = (f) => { let x = f; const seen = new Set(); while (merged[x] && !seen.has(x)) { seen.add(x); x = merged[x]; } return x; };

const anchorCache = new Map();
const anchorsOf = (f) => {
  if (!anchorCache.has(f)) {
    const t = fs.existsSync(abs(f)) ? fs.readFileSync(abs(f), 'utf8') : '';
    anchorCache.set(f, new Set([...t.matchAll(/^#{1,6}\s+(.+)$/gm)].map((m) => slug(m[1]))));
  }
  return anchorCache.get(f);
};

let changed = 0, dropped = 0;
for (const f of files) {
  if (merged[f]) continue;
  const t = fs.readFileSync(abs(f), 'utf8');
  const out = t.replace(/(\]\()([^)\s]+?)(\))/g, (m, a, target, c) => {
    if (/^(https?:|mailto:|#)/.test(target)) return m;
    const [p, ...rest] = target.split('#');
    const anchor = rest.join('#');
    const tgt = rel(path.resolve(path.dirname(abs(f)), p));
    if (!merged[tgt]) return m;
    const dest = finalOf(tgt);
    let r = path.relative(path.dirname(abs(f)), abs(dest)).split(path.sep).join('/');
    if (!r) r = path.basename(dest);
    if (anchor && anchorsOf(dest).has(decodeURIComponent(anchor))) return a + r + '#' + anchor + c;
    if (anchor) dropped++;
    return a + r + c;
  }).replace(/docs\/[A-Za-z0-9_\-./]+\.md/g, (s) => (merged[s] ? finalOf(s) : s));
  if (out !== t) { fs.writeFileSync(abs(f), out); changed++; }
}
for (const f of walk(abs('specs'), ['.yaml', '.yml', '.json', '.md']).map(rel)) {
  const t = fs.readFileSync(abs(f), 'utf8');
  const o = t.replace(/docs\/[A-Za-z0-9_\-./]+\.md/g, (s) => (merged[s] ? finalOf(s) : s));
  if (o !== t) { fs.writeFileSync(abs(f), o); changed++; }
}
for (const f of Object.keys(merged)) fs.unlinkSync(abs(f));
console.log(`redirect-merged: xóa ${Object.keys(merged).length} file, cập nhật ${changed} file, bỏ ${dropped} anchor không còn tồn tại`);
console.log(Object.entries(merged).map(([a, b]) => `  ${a} -> ${b}`).join('\n'));
