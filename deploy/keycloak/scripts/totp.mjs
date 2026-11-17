// RFC 6238 TOTP (HMAC-SHA1, 6 digits, 30 s) for tests against the local realm.
import { createHmac } from 'node:crypto';

export function totp(secret, at = Date.now(), period = 30, digits = 6) {
  const counter = Buffer.alloc(8);
  counter.writeBigUInt64BE(BigInt(Math.floor(at / 1000 / period)));
  const h = createHmac('sha1', Buffer.from(secret, 'utf8')).update(counter).digest();
  const o = h[h.length - 1] & 0xf;
  const n = ((h[o] & 0x7f) << 24) | (h[o + 1] << 16) | (h[o + 2] << 8) | h[o + 3];
  return String(n % 10 ** digits).padStart(digits, '0');
}
