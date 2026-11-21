// Breaking-change exceptions of `make specs-breaking` (ADR-0095 mục 7): oasdiff still reports every breaking change; a
// change is accepted only when an entry of specs/breaking-allowlist.yaml names it exactly (spec file, oasdiff check id,
// HTTP method, path and, when given, a fragment of the message) and cites an existing ADR. Everything else fails the
// check. Pure functions, unit-tested by breaking-allowlist.test.mjs.

/** Normalizes a spec for the OpenAPI 3.0 loader of oasdiff: numeric exclusiveMinimum/exclusiveMaximum (OpenAPI 3.1 /
 * JSON Schema 2020-12) become minimum/maximum + boolean flag, which is the same constraint; a boolean `items` (tuples
 * with prefixItems: `items: false`) is dropped, as 3.0 has no tuple. Mutates and returns doc. */
export function normalizeFor30(doc) {
  const walk = (n) => {
    if (Array.isArray(n)) {
      n.forEach(walk);
      return;
    }
    if (!n || typeof n !== 'object') return;
    if (typeof n.items === 'boolean') delete n.items;
    for (const [k, b] of [['exclusiveMinimum', 'minimum'], ['exclusiveMaximum', 'maximum']]) {
      if (typeof n[k] === 'number') {
        n[b] = n[k];
        n[k] = true;
      }
    }
    Object.values(n).forEach(walk);
  };
  walk(doc);
  return doc;
}

const ADR = /^ADR-\d{4}$/;
const METHOD = /^(GET|PUT|POST|DELETE|PATCH|HEAD|OPTIONS|TRACE)$/;
const FIELDS = ['adr', 'reason', 'id', 'path'];

// Files of an entry: one spec path or a list (the public bundle and the module file split from it report the same change).
const filesOf = (e) => (Array.isArray(e?.file) ? e.file : [e?.file]);

/** Validates the allowlist; returns a list of problems (empty when valid). adrExists(adr) tells whether the ADR file exists. */
export function validateAllowlist(entries, adrExists) {
  if (!Array.isArray(entries)) return ['allowlist: `exceptions` must be a list'];
  const problems = [];
  entries.forEach((e, i) => {
    const at = `exception #${i + 1}`;
    for (const f of FIELDS) {
      if (typeof e?.[f] !== 'string' || e[f].trim() === '') problems.push(`${at}: missing ${f}`);
    }
    if (!filesOf(e).every((f) => typeof f === 'string' && f.startsWith('specs/openapi/'))) {
      problems.push(`${at}: file must be a spec path under specs/openapi/ (or a list of them)`);
    }
    if (typeof e?.adr === 'string') {
      if (!ADR.test(e.adr)) problems.push(`${at}: adr must look like ADR-0095`);
      else if (!adrExists(e.adr)) problems.push(`${at}: ${e.adr} has no file in docs/adr`);
    }
    if (e?.operation !== undefined && !METHOD.test(e.operation)) problems.push(`${at}: operation must be an upper-case HTTP method`);
    if (typeof e?.path === 'string' && /[*?]/.test(e.path)) problems.push(`${at}: path must be exact (no wildcard)`);
    if (e?.text !== undefined && (typeof e.text !== 'string' || e.text === '')) problems.push(`${at}: text must be a non-empty string`);
  });
  return problems;
}

/** Whether entry accepts the oasdiff change c of spec file (repository-relative path). */
export function matches(entry, file, c) {
  if (!filesOf(entry).includes(file) || entry.id !== c.id || entry.path !== c.path) return false;
  if (entry.operation !== undefined && entry.operation !== c.operation) return false;
  if (entry.text !== undefined && !String(c.text ?? '').includes(entry.text)) return false;
  return true;
}

/** Splits the error-level changes of file into accepted (with their entry) and rejected. level 3 = ERR in oasdiff JSON. */
export function classify(entries, file, changes) {
  const accepted = [];
  const rejected = [];
  for (const c of changes) {
    if (c.level !== undefined && c.level < 3) continue;
    const e = entries.find((x) => matches(x, file, c));
    if (e) accepted.push({ change: c, entry: e });
    else rejected.push(c);
  }
  return { accepted, rejected };
}

/** One line per change, for logs. */
export function describe(file, c) {
  return `${file} [${c.id}] ${c.operation ?? ''} ${c.path ?? ''}: ${c.text ?? ''}`.replace(/\s+/g, ' ').trim();
}
