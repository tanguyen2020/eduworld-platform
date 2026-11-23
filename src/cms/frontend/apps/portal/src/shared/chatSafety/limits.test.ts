// F122-AC-26 (ADR-0041, AGENTS R33): the field limits of the chat-safety screens equal the OpenAPI spec they mirror.
import { describe, expect, it } from 'vitest';
import { locate } from '../../test/openapiLocate';
import { LIMITS } from './ui';

const SERVER_RULES = new Set(['switchReason.minLength']); // ≥ 10 enforced by the server (MODERATION_SWITCH_REASON_REQUIRED), not in the schema
const UUID_SOURCE = 'schemas/Uuid';

describe('chat-safety LIMITS mirror the OpenAPI spec (F122-AC-26)', () => {
  for (const [name, l] of Object.entries(LIMITS)) {
    it(name, () => {
      if (l.source === UUID_SOURCE) {
        expect(locate(l.source).format).toBe('uuid');
        return;
      }
      const s = locate(l.source);
      expect(s.maxLength, `${name}.maxLength`).toBe(l.maxLength);
      if (!SERVER_RULES.has(`${name}.minLength`)) expect(s.minLength, `${name}.minLength`).toBe('minLength' in l ? l.minLength : undefined);
      if ('pattern' in l) expect(s.pattern, `${name}.pattern`).toBe(l.pattern);
    });
  }
});
