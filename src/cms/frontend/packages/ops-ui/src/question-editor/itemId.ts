// itemId (F053-AC-02, QUESTION_BANK §2, research R4): short random ids of options / blanks / zones… stable within a
// version and never revealing the order (no A/B/C/D, no index): `<prefix>_<base36>`, pattern of
// question.schema.json `$defs/itemId` = ^[a-z]_[a-z0-9_]{1,24}$.

export const ITEM_ID_PATTERN = /^[a-z]_[a-z0-9_]{1,24}$/;

/** Prefixes by kind: o option, s statement, b blank, l / r matching columns, i ordering item, d draggable,
 * z zone, h hotspot region, p label, x drop point. */
export type ItemPrefix = 'o' | 's' | 'b' | 'l' | 'r' | 'i' | 'd' | 'z' | 'h' | 'p' | 'x';

export type RandomBytes = (n: number) => Uint8Array;

const defaultRandom: RandomBytes = (n) => crypto.getRandomValues(new Uint8Array(n));

/** A new id not in `existing` (4 random bytes → up to 7 base36 characters). */
export function itemId(prefix: ItemPrefix, existing: Iterable<string> = [], random: RandomBytes = defaultRandom): string {
  const taken = new Set(existing);
  for (let attempt = 0; ; attempt++) {
    const b = random(4);
    const n = ((b[0]! << 24) >>> 0) + (b[1]! << 16) + (b[2]! << 8) + b[3]!;
    const id = `${prefix}_${n.toString(36).padStart(4, '0')}`;
    if (!taken.has(id) && ITEM_ID_PATTERN.test(id)) return id;
    if (attempt > 1000) throw new Error('itemId: no free id');
  }
}

export function isItemId(v: unknown): v is string {
  return typeof v === 'string' && ITEM_ID_PATTERN.test(v);
}
