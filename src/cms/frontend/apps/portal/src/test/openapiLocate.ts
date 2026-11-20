// Test helper (ADR-0041, AGENTS R33): finds a schema in the merged OpenAPI spec so the field limits mirrored by the
// portal (`fieldSchemas.ts` of a workspace) can be compared with the contract, which stays the single source.
import fs from 'node:fs';
import path from 'node:path';
import yaml from 'js-yaml';

type Node = Record<string, unknown>;

const REPO = path.resolve(__dirname, '../../../../../../..');
const spec = yaml.load(fs.readFileSync(path.join(REPO, 'specs/openapi/eduworld-api.v1.yaml'), 'utf8')) as Node;
const components = spec.components as Record<string, Record<string, Node>>;

function deref(n: Node | undefined): Node {
  let cur = n ?? {};
  while (typeof cur.$ref === 'string') cur = (cur.$ref as string).slice(2).split('/').reduce<Node>((o, k) => o[k] as Node, spec);
  return cur;
}

function operation(id: string): Node {
  const op = Object.values(spec.paths as Node)
    .flatMap((item) => Object.values(item as Node) as Node[])
    .find((o) => o?.operationId === id);
  if (!op) throw new Error(`operationId ${id} not found`);
  return op;
}

/**
 * Schema at `schemas/X/properties/y`, `parameters/X`, `paths/<operationId>/<query param>` or
 * `paths/<operationId>/body/<property>` (JSON request body); a nullable oneOf is flattened.
 */
export function locate(source: string): Node {
  const [kind, a, b, c] = source.split('/');
  let s: Node;
  if (kind === 'schemas') s = deref(b === 'properties' ? ((deref(components.schemas![a!]).properties as Node)[c!] as Node) : components.schemas![a!]);
  else if (kind === 'parameters') s = deref(components.parameters![a!]!.schema as Node);
  else if (b === 'body') {
    const body = deref(operation(a!).requestBody as Node);
    const schema = deref(((body.content as Node)['application/json'] as Node).schema as Node);
    s = deref((schema.properties as Node)[c!] as Node);
  } else s = deref(((operation(a!).parameters as Node[]).map(deref).find((p) => p.name === b) as Node).schema as Node);
  const variants = (s.oneOf as Node[] | undefined)?.map(deref).filter((v) => v.type !== 'null');
  return variants?.length === 1 ? { ...variants[0] } : s;
}

/** `enum` of `schemas/X` (after $ref). */
export function enumOf(source: string): unknown[] {
  return (locate(source).enum as unknown[] | undefined) ?? [];
}
