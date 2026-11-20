// F052-AC-26 (ADR-0041): the form limits of fieldSchemas.ts equal the OpenAPI spec they mirror.
import { describe, expect, it } from 'vitest';
import { locate } from '../../../test/openapiLocate';
import { FIELDS, LIMITS } from './fieldSchemas';

describe('fieldSchemas mirror the OpenAPI spec (ADR-0041)', () => {
  for (const [name, f] of Object.entries(FIELDS)) {
    it(name, () => {
      const s = locate(f.source);
      expect(s.maxLength, `${name}.maxLength`).toBe(f.maxLength);
      expect(s.minLength, `${name}.minLength`).toBe('minLength' in f ? f.minLength : undefined);
      expect(s.pattern, `${name}.pattern`).toBe('pattern' in f ? f.pattern : undefined);
    });
  }
  for (const [name, l] of Object.entries(LIMITS)) {
    it(name, () => {
      const s = locate(l.source);
      for (const k of ['minimum', 'maximum', 'maxItems'] as const) if (k in l) expect(s[k], `${name}.${k}`).toBe((l as unknown as Record<string, number>)[k]);
    });
  }
});
