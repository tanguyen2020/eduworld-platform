// Tách docs/02-architecture/ARCHITECTURE_DECISIONS.md thành docs/adr/ADR-NNNN-<slug>.md + README.md + TEMPLATE.md
// và viết lại mọi link "ARCHITECTURE_DECISIONS.md#adr-NNNN-..." sang file ADR tương ứng.
import fs from 'node:fs';
import path from 'node:path';
import { abs, applyMoves } from './lib-links.mjs';

const SRC = 'docs/02-architecture/ARCHITECTURE_DECISIONS.md';
const text = fs.readFileSync(abs(SRC), 'utf8');

const ascii = (s) => s.normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/đ/g, 'd').replace(/Đ/g, 'D');
const slugify = (s) => ascii(s).toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '').slice(0, 60).replace(/-$/, '');

const headRe = /^## (ADR-(\d{4})) — (.+)$/gm;
const heads = [...text.matchAll(headRe)];
const templateIdx = text.indexOf('## Mẫu ADR mới');
const indexRows = new Map();
for (const m of text.matchAll(/^\| \[(\d{4})\]\([^)]*\) \| ([^|]+)\| ([^|]+)\| ([^|]+)\|$/gm)) {
  indexRows.set(m[1], { title: m[2].trim(), status: m[3].trim(), oq: m[4].trim() });
}

fs.mkdirSync(abs('docs/adr'), { recursive: true });
const files = {};
// Tính trước tên file để chuyển link nội bộ "#adr-NNNN-..."
heads.forEach((h) => { files[h[2]] = `docs/adr/ADR-${h[2]}-${slugify(h[3])}.md`; });
const OLD_DIR = path.dirname(abs(SRC));
const NEW_DIR = abs('docs/adr');
const relink = (s) => s.replace(/(\]\()([^)\s]+?)(\))/g, (m, a, t, c) => {
  if (/^(https?:|mailto:)/.test(t)) return m;
  const am = t.match(/^#adr-(\d{4})/);
  if (am && files[am[1]]) return a + files[am[1]].split('/').pop() + c;
  if (t.startsWith('#')) return m;
  const [p, ...rest] = t.split('#');
  const anchor = rest.length ? '#' + rest.join('#') : '';
  const target = path.resolve(OLD_DIR, p);
  if (target === abs(SRC)) {
    const m2 = anchor.match(/^#adr-(\d{4})/);
    return a + (m2 && files[m2[1]] ? files[m2[1]].split('/').pop() : 'README.md') + c;
  }
  return a + path.relative(NEW_DIR, target).split(path.sep).join('/') + anchor + c;
});
heads.forEach((h, i) => {
  const start = h.index;
  let end = i + 1 < heads.length ? heads[i + 1].index : templateIdx;
  let body = text.slice(start, end).replace(/\n---\s*\n*$/, '\n').trimEnd() + '\n';
  const num = h[2];
  const file = files[num];
  body = relink(body).replace(/^## (ADR-\d{4} — .+)$/m, '# $1');
  const meta = indexRows.get(num);
  const header = `\n> Trạng thái (chỉ mục): ${meta ? meta.status : 'xem nội dung'} · Liên quan OQ: ${meta ? meta.oq : '—'} · [Chỉ mục ADR](README.md)\n`;
  body = body.replace(/^(# ADR-\d{4} — .+)\n/, `$1\n${header}`);
  fs.writeFileSync(abs(file), body);
});

// TEMPLATE
const tpl = relink(text.slice(templateIdx).replace('## Mẫu ADR mới', '# Mẫu ADR').trim()) + '\n';
fs.writeFileSync(abs('docs/adr/TEMPLATE.md'), tpl);

// README: phần đầu (trước "## Chỉ mục") + bảng chỉ mục trỏ file
const preamble = relink(text.slice(0, text.indexOf('## Chỉ mục')).replace(/^# .+$/m, '# Architecture Decision Records (ADR)').trimEnd());
let table = '## Chỉ mục\n\n| ADR | Tiêu đề | Trạng thái | Phụ thuộc OQ |\n|---|---|---|---|\n';
for (const num of Object.keys(files).sort()) {
  const m = indexRows.get(num);
  const name = files[num].split('/').pop();
  table += `| [${num}](${name}) | ${m ? m.title : ''} | ${m ? m.status : ''} | ${m ? m.oq : '—'} |\n`;
}
const readme = `${preamble}\n\n- Mỗi ADR là một file \`ADR-NNNN-<slug>.md\`. Mẫu: [TEMPLATE.md](TEMPLATE.md).\n\n${table}`;
fs.writeFileSync(abs('docs/adr/README.md'), readme);

// Viết lại link (kể cả link nội bộ trong ADR: "#adr-NNNN-...")
const mapper = (target, anchor) => {
  if (target !== SRC) return null;
  const m = anchor.match(/^#adr-(\d{4})/);
  if (m && files[m[1]]) return { path: files[m[1]], anchor: '' };
  return { path: 'docs/adr/README.md', anchor: '' };
};
const changed = applyMoves({}, mapper);
fs.unlinkSync(abs(SRC));
console.log(`split-adr: ${Object.keys(files).length} ADR, ${changed} file cập nhật link`);
