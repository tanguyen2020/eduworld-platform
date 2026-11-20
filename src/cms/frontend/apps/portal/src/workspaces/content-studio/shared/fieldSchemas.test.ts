// F054-AC-13, AC-24, AC-29 (ADR-0041): the limits and enums of fieldSchemas.ts equal the OpenAPI spec they mirror.
import { describe, expect, it } from 'vitest';
import { locate } from '../../../test/openapiLocate';
import { ENUMS, FIELDS, LIMITS } from './fieldSchemas';

describe('content-studio fieldSchemas mirror the OpenAPI spec (ADR-0041)', () => {
  for (const [name, f] of Object.entries(FIELDS)) {
    it(name, () => {
      const s = locate(f.source);
      expect(s.maxLength, `${name}.maxLength`).toBe(f.maxLength);
      expect(s.minLength, `${name}.minLength`).toBe('minLength' in f ? f.minLength : undefined);
    });
  }
  for (const [name, l] of Object.entries(LIMITS)) {
    it(name, () => {
      expect(locate(l.source).maxItems, `${name}.maxItems`).toBe(l.maxItems);
    });
  }
  for (const [name, e] of Object.entries(ENUMS)) {
    it(name, () => {
      expect([...(locate(e.source).enum as string[])].sort()).toEqual([...e.values].sort());
    });
  }
});
