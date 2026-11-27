// Spectral custom function — ANTI_CHEAT §4, ADR-0016, AGENTS R5 (F132-AC-21): the request body of a student operation
// never declares a value the server must decide. Forbidden property names at any depth: score, xp, is_correct, rating.
// Student operation = a player route (/learning, /exams, /game — not /game-admin —, /multiplayer, /competition)
// that is not a portal action (x-portal-action) nor an internal-gateway route (x-internal). Responses are not checked.
//
// The same logic runs without the Spectral CLI in `make specs-lint` (scripts/specs/check-client-authoritative.mjs),
// which resolves local $ref itself; Spectral hands this function an already resolved operation.
export const FORBIDDEN = ['score', 'xp', 'is_correct', 'rating'];
export const STUDENT_PREFIXES = ['/learning/', '/exams/', '/game/', '/multiplayer/', '/competition/'];
const BODY_METHODS = ['post', 'put', 'patch'];

/** True when the path + operation is a student (game player) operation. */
export function isStudentOperation(pathKey, operation) {
  if (!operation || typeof operation !== 'object') return false;
  if (operation['x-portal-action'] || operation['x-internal'] === true) return false;
  const p = String(pathKey);
  return STUDENT_PREFIXES.some((pre) => p.startsWith(pre) || p === pre.slice(0, -1));
}

/**
 * Returns [{field, pointer}] for every forbidden property declared in the request body schemas of operation.
 * resolve(node) maps a {$ref} node to its target (identity when refs are already resolved).
 */
export function forbiddenRequestFields(operation, resolve = (n) => n) {
  const out = [];
  const seen = new Set();
  const content = resolve(operation?.requestBody)?.content ?? {};
  const walk = (raw, ptr) => {
    const node = resolve(raw);
    if (!node || typeof node !== 'object' || seen.has(node)) return;
    seen.add(node);
    if (Array.isArray(node)) {
      node.forEach((n, i) => walk(n, `${ptr}/${i}`));
      return;
    }
    for (const [name, sub] of Object.entries(node.properties ?? {})) {
      if (FORBIDDEN.includes(name)) out.push({ field: name, pointer: `${ptr}/properties/${name}` });
      walk(sub, `${ptr}/properties/${name}`);
    }
    for (const req of Array.isArray(node.required) ? node.required : []) {
      if (FORBIDDEN.includes(req) && !(node.properties && req in node.properties)) out.push({ field: req, pointer: `${ptr}/required` });
    }
    for (const k of ['items', 'additionalProperties', 'not']) if (node[k] && typeof node[k] === 'object') walk(node[k], `${ptr}/${k}`);
    for (const k of ['allOf', 'oneOf', 'anyOf']) if (Array.isArray(node[k])) node[k].forEach((n, i) => walk(n, `${ptr}/${k}/${i}`));
  };
  for (const [media, m] of Object.entries(content)) walk(m?.schema, `/requestBody/content/${media}/schema`);
  return out;
}

/** Spectral entry point; given: "$.paths[*][post,put,patch]". */
export default function noClientAuthoritativeFields(targetVal, _opts, context) {
  const path = context?.path ?? [];
  const pathKey = path[path.length - 2];
  const method = path[path.length - 1];
  if (!BODY_METHODS.includes(String(method)) || !isStudentOperation(pathKey, targetVal)) return [];
  return forbiddenRequestFields(targetVal).map((f) => ({
    message: `Request của học sinh khai báo trường "${f.field}" do server quyết định (ANTI_CHEAT §4, ADR-0016).`,
    path: [...path, ...f.pointer.split('/').slice(1)],
  }));
}
