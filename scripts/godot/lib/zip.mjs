// Minimal ZIP reader/writer for the release scripts (APK, AAB, IPA are ZIP files): no dependency, no zip64
// (artifacts stay far below 4 GiB, PERFORMANCE_BUDGET §11). The writer copies existing entries byte for byte
// (`raw`), stores new ones uncompressed or deflated, and pads stored entries to a 4-byte data offset like
// `zipalign -p 4` (a re-sign with apksigner is still required after changing an APK — F133 T039).
import zlib from 'node:zlib';

const SIG_LOCAL = 0x04034b50;
const SIG_CENTRAL = 0x02014b50;
const SIG_EOCD = 0x06054b50;

export function readZip(buf) {
  let eocd = -1;
  for (let i = buf.length - 22; i >= Math.max(0, buf.length - 22 - 0xffff); i--) {
    if (buf.readUInt32LE(i) === SIG_EOCD) { eocd = i; break; }
  }
  if (eocd < 0) throw new Error('zip: end of central directory not found (not a ZIP file)');
  const count = buf.readUInt16LE(eocd + 10);
  let p = buf.readUInt32LE(eocd + 16);
  const entries = [];
  for (let n = 0; n < count; n++) {
    if (buf.readUInt32LE(p) !== SIG_CENTRAL) throw new Error('zip: bad central directory');
    const method = buf.readUInt16LE(p + 10);
    const crc = buf.readUInt32LE(p + 16);
    const compSize = buf.readUInt32LE(p + 20);
    const size = buf.readUInt32LE(p + 24);
    const nameLen = buf.readUInt16LE(p + 28);
    const extraLen = buf.readUInt16LE(p + 30);
    const commentLen = buf.readUInt16LE(p + 32);
    const offset = buf.readUInt32LE(p + 42);
    const name = buf.subarray(p + 46, p + 46 + nameLen).toString('utf8');
    p += 46 + nameLen + extraLen + commentLen;
    const local = offset;
    if (buf.readUInt32LE(local) !== SIG_LOCAL) throw new Error(`zip: bad local header for ${name}`);
    const start = local + 30 + buf.readUInt16LE(local + 26) + buf.readUInt16LE(local + 28);
    const comp = buf.subarray(start, start + compSize);
    entries.push({
      name, method, crc, size, compSize,
      raw: { comp, crc, size, method },
      data() {
        if (method === 0) return Buffer.from(comp);
        if (method === 8) return zlib.inflateRawSync(comp);
        throw new Error(`zip: unsupported method ${method} for ${name}`);
      },
    });
  }
  return entries;
}

/** entries: [{name, data?: Buffer, deflate?: boolean, raw?: {comp, crc, size, method}}] → ZIP Buffer. */
export function writeZip(entries) {
  const parts = [];
  const central = [];
  let offset = 0;
  for (const e of entries) {
    const nameBuf = Buffer.from(e.name, 'utf8');
    let comp, crc, size, method;
    if (e.raw) ({ comp, crc, size, method } = e.raw);
    else {
      size = e.data.length;
      crc = zlib.crc32(e.data) >>> 0;
      method = e.deflate ? 8 : 0;
      comp = e.deflate ? zlib.deflateRawSync(e.data) : e.data;
    }
    let pad = 0;
    if (method === 0) pad = (4 - ((offset + 30 + nameBuf.length) % 4)) % 4;
    const local = Buffer.alloc(30);
    local.writeUInt32LE(SIG_LOCAL, 0);
    local.writeUInt16LE(20, 4);
    local.writeUInt16LE(0x0800, 6); // UTF-8 names
    local.writeUInt16LE(method, 8);
    local.writeUInt32LE(0, 10); // time/date: fixed for reproducible output
    local.writeUInt32LE(crc, 14);
    local.writeUInt32LE(comp.length, 18);
    local.writeUInt32LE(size, 22);
    local.writeUInt16LE(nameBuf.length, 26);
    local.writeUInt16LE(pad, 28);
    parts.push(local, nameBuf, Buffer.alloc(pad), comp);
    const c = Buffer.alloc(46);
    c.writeUInt32LE(SIG_CENTRAL, 0);
    c.writeUInt16LE(20, 4);
    c.writeUInt16LE(20, 6);
    c.writeUInt16LE(0x0800, 8);
    c.writeUInt16LE(method, 10);
    c.writeUInt32LE(0, 12);
    c.writeUInt32LE(crc, 16);
    c.writeUInt32LE(comp.length, 20);
    c.writeUInt32LE(size, 24);
    c.writeUInt16LE(nameBuf.length, 28);
    c.writeUInt32LE(offset, 42);
    central.push(c, nameBuf);
    offset += 30 + nameBuf.length + pad + comp.length;
  }
  const cdSize = central.reduce((s, b) => s + b.length, 0);
  const eocd = Buffer.alloc(22);
  eocd.writeUInt32LE(SIG_EOCD, 0);
  eocd.writeUInt16LE(entries.length, 8);
  eocd.writeUInt16LE(entries.length, 10);
  eocd.writeUInt32LE(cdSize, 12);
  eocd.writeUInt32LE(offset, 16);
  return Buffer.concat([...parts, ...central, eocd]);
}

/** True when `needle` occurs in `buf` as UTF-8 or UTF-16LE (Android binary XML string pools use UTF-16). */
export function containsText(buf, needle) {
  return buf.includes(Buffer.from(needle, 'utf8')) || buf.includes(Buffer.from(needle, 'utf16le'));
}
