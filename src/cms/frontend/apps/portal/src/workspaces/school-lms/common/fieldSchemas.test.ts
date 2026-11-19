// F052-AC-26 (ADR-0041): the form limits of fieldSchemas.ts equal the OpenAPI spec they mirror.
import fs from 'node:fs';
import path from 'node:path';
import yaml from 'js-yaml';
import { describe, expect, it } from 'vitest';
import { FIELDS, LIMITS } from './fieldSchemas';

type Node = Record<string, unknown>;
const REPO = path.resolve(__dirname, '../../../../../../../../..');
const spec = yaml.load(fs.readFileSync(path.join(REPO, 'specs/openapi/eduworld-api.v1.yaml'), 'utf8')) as Node;
const components = spec.components as Record<string, Record<string, Node>>;

function deref(n: Node | undefined): Node {
  let cur = n ?? {};
  while (typeof cur.$ref === 'string') cur = (cur.$ref as string).slice(2).split('/').reduce<Node>((o, k) => o[k] as Node, spec);
  return cur;
}

/** Schema at `schemas/X/properties/y`, `parameters/X` or `paths/<operationId>/<query param>`; nullable oneOf flattened. */
function locate(source: string): Node {
  const [kind, a, b, c] = source.split('/');
  let s: Node;
  if (kind === 'schemas') s = deref(b === 'properties' ? (deref(components.schemas![a!]).properties as Node)[c!] as Node : components.schemas![a!]);
  else if (kind === 'parameters') s = deref(components.parameters![a!]!.schema as Node);
  else {
    const op = Object.values(spec.paths as Node).flatMap((item) => Object.values(item as Node) as Node[]).find((o) => o?.operationId === a)!;
    s = deref(((op.parameters as Node[]).map(deref).find((p) => p.name === b) as Node).schema as Node);
  }
  const variants = (s.oneOf as Node[] | undefined)?.map(deref).filter((v) => v.type !== 'null');
  return variants?.length === 1 ? { ...variants[0] } : s;
}

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
