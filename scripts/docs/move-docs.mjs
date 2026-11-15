// Di chuyển/đổi tên tài liệu theo bản đồ JSON và viết lại mọi link.
// Dùng: node docs/move-docs.mjs <map.json>   (map: { "docs/old.md": "docs/new.md" })
import fs from 'node:fs';
import { abs, applyMoves } from './lib-links.mjs';

const mapFile = process.argv[2];
if (!mapFile) { console.error('usage: node docs/move-docs.mjs <map.json>'); process.exit(2); }
const map = JSON.parse(fs.readFileSync(mapFile, 'utf8'));
for (const [o, n] of Object.entries(map)) {
  if (!fs.existsSync(abs(o))) { console.error(`missing: ${o}`); process.exit(1); }
  if (fs.existsSync(abs(n)) && o !== n) { console.error(`target exists: ${n}`); process.exit(1); }
}
const changed = applyMoves(map);
console.log(`move-docs: ${Object.keys(map).length} file di chuyển, ${changed} file cập nhật`);
