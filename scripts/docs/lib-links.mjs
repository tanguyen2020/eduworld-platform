// Tiện ích dùng chung: duyệt file Markdown và viết lại link tương đối khi file được di chuyển.
import fs from 'node:fs';
import path from 'node:path';

export const ROOT = path.resolve(path.dirname(new URL(import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, '$1')), '..', '..');

const SKIP = new Set(['node_modules', '.git', '.claude']);

export function walk(dir, exts = ['.md']) {
  const out = [];
  for (const name of fs.readdirSync(dir)) {
    if (SKIP.has(name)) continue;
    const p = path.join(dir, name);
    const st = fs.statSync(p);
    if (st.isDirectory()) out.push(...walk(p, exts));
    else if (exts.some((e) => p.endsWith(e))) out.push(p);
  }
  return out;
}

export const rel = (p) => path.relative(ROOT, p).split(path.sep).join('/');
export const abs = (r) => path.join(ROOT, ...r.split('/'));

const LINK_RE = /(\]\()([^)\s]+?)(\))/g;

/**
 * Viết lại link trong toàn bộ file .md của repo.
 * @param {Record<string,string>} fileMap  repo-relative cũ -> mới (file được di chuyển)
 * @param {(oldTarget:string, anchor:string)=>{path:string, anchor:string}|null} [linkMapper] ánh xạ tùy chỉnh (vd ADR anchor)
 */
export function applyMoves(fileMap, linkMapper) {
  const files = walk(ROOT).map(rel);
  const contents = new Map(files.map((f) => [f, fs.readFileSync(abs(f), 'utf8')]));
  const newLoc = (f) => fileMap[f] ?? f;
  let changed = 0;
  for (const [oldF, text] of contents) {
    const newF = newLoc(oldF);
    const out = text.replace(LINK_RE, (m, a, target, c) => {
      if (/^(https?:|mailto:|#)/.test(target)) return m;
      const [p, ...rest] = target.split('#');
      let anchor = rest.length ? '#' + rest.join('#') : '';
      const oldTarget = rel(path.resolve(path.dirname(abs(oldF)), p));
      let newTarget = newLoc(oldTarget);
      if (linkMapper) {
        const mapped = linkMapper(oldTarget, anchor);
        if (mapped) { newTarget = mapped.path; anchor = mapped.anchor; }
      }
      if (newTarget === oldTarget && newF === oldF) return m;
      let r = path.relative(path.dirname(abs(newF)), abs(newTarget)).split(path.sep).join('/');
      if (!r) r = path.basename(newTarget);
      return a + r + anchor + c;
    })
      // đường dẫn dạng chữ "docs/xxx.md" trong văn bản/backtick
      .replace(/docs\/[A-Za-z0-9_\-./]+\.md/g, (s) => (fileMap[s] ? fileMap[s] : s));
    if (newF !== oldF) {
      fs.mkdirSync(path.dirname(abs(newF)), { recursive: true });
      fs.writeFileSync(abs(newF), out);
      fs.unlinkSync(abs(oldF));
      changed++;
    } else if (out !== text) {
      fs.writeFileSync(abs(oldF), out);
      changed++;
    }
  }
  // spec/yaml/json: chỉ thay chuỗi đường dẫn docs/
  for (const f of walk(abs('specs'), ['.yaml', '.yml', '.json', '.md']).map(rel)) {
    const t = fs.readFileSync(abs(f), 'utf8');
    const o = t.replace(/docs\/[A-Za-z0-9_\-./]+\.md/g, (s) => (fileMap[s] ? fileMap[s] : s));
    if (o !== t) { fs.writeFileSync(abs(f), o); changed++; }
  }
  return changed;
}
