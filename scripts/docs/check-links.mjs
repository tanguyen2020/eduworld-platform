// Kiểm tra link tương đối và anchor (#...) trong mọi file .md của repo (bỏ qua node_modules).
// Anchor tính theo quy tắc slug GitHub: chữ thường, giữ chữ/số Unicode (kể cả tiếng Việt có dấu), '_' và '-',
// bỏ ký tự khác, khoảng trắng → '-', heading trùng thêm hậu tố -1, -2...
// Chạy: node scripts/docs/check-links.mjs [repoRoot] → exit 1 nếu có link/anchor hỏng.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const repoRoot = path.resolve(process.argv[2] ?? path.join(path.dirname(fileURLToPath(import.meta.url)), '..', '..'));
const SKIP = new Set(['node_modules', '.git', '.claude']);
const mdFiles = [];
function walk(dir) {
  for (const name of fs.readdirSync(dir)) {
    const p = path.join(dir, name);
    const st = fs.statSync(p);
    // .specify/templates: link tương đối chỉ đúng khi template được sao vào specs/<NNN>-<slug>/ (ADR-0082)
    if (st.isDirectory()) { if (!SKIP.has(name) && !p.endsWith(path.join('.specify', 'templates'))) walk(p); }
    else if (name.endsWith('.md')) mdFiles.push(p);
  }
}
walk(repoRoot);

const slug = (h) => h.trim().toLowerCase()
  .replace(/<[^>]+>/g, '')
  .replace(/[`*_~]{2,}|[`*~]/g, (s) => (s.includes('_') ? s : ''))
  .replace(/[^\p{L}\p{N}\p{M}\s_-]/gu, '')
  .replace(/\s/g, '-');

const anchorCache = new Map();
function anchorsOf(file) {
  if (anchorCache.has(file)) return anchorCache.get(file);
  const set = new Set();
  const counts = new Map();
  let inFence = false;
  for (const line of fs.readFileSync(file, 'utf8').split('\n')) {
    if (/^\s*```/.test(line)) { inFence = !inFence; continue; }
    if (inFence) continue;
    const m = line.match(/^#{1,6}\s+(.*?)\s*#*\s*$/);
    if (!m) continue;
    const base = slug(m[1]);
    const n = counts.get(base) ?? 0;
    counts.set(base, n + 1);
    set.add(n === 0 ? base : `${base}-${n}`);
  }
  anchorCache.set(file, set);
  return set;
}

let links = 0;
const broken = [];
for (const file of mdFiles) {
  let inFence = false;
  const lines = fs.readFileSync(file, 'utf8').split('\n');
  lines.forEach((line, i) => {
    if (/^\s*```/.test(line)) { inFence = !inFence; return; }
    if (inFence) return;
    const re = /\[[^\]]*\]\(([^)\s]+)(?:\s+"[^"]*")?\)/g;
    let m;
    while ((m = re.exec(line))) {
      const target = m[1];
      if (/^(https?:|mailto:|tel:)/i.test(target)) continue;
      links++;
      const [rawPath, rawAnchor] = target.split('#');
      const resolved = rawPath ? path.resolve(path.dirname(file), decodeURI(rawPath)) : file;
      if (!fs.existsSync(resolved)) { broken.push(`${path.relative(repoRoot, file)}:${i + 1} → ${target} (không có file)`); continue; }
      if (rawAnchor && resolved.endsWith('.md')) {
        const a = decodeURIComponent(rawAnchor).toLowerCase();
        if (!anchorsOf(resolved).has(a)) broken.push(`${path.relative(repoRoot, file)}:${i + 1} → ${target} (không có anchor)`);
      }
    }
  });
}
console.log(`check-links: ${mdFiles.length} file, ${links} link nội bộ, ${broken.length} hỏng`);
if (broken.length) { console.log(broken.join('\n')); process.exit(1); }
