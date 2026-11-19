// Contract fixtures for the game client (F060 T041, F060-AC-22, GODOT_ARCHITECTURE §12).
// Generates src/game/tests/fixtures/contract/ from the single sources of truth:
//   - openapi/<Schema>.json: the `example` of each component schema the client parses, or a deterministic
//     sample built from the schema when the spec has no example (required + optional fields, first enum value);
//   - examples/<file>.json: copies of specs/examples/*.json (content documents the client will load);
//   - manifest.json: source file + sha256 of every fixture (generated marker, checked in CI).
// Usage: node src/game/tools/gen_fixtures.mjs [--check]   (--check exits 1 when fixtures are stale)
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..');
const yaml = require(path.join(root, 'scripts', 'node_modules', 'js-yaml'));

const SPEC = 'specs/openapi/eduworld-api.v1.yaml';
const EXAMPLES_DIR = 'specs/examples';
const OUT_DIR = path.join(root, 'src/game/tests/fixtures/contract');
// Component schemas the F060 frame parses (F061 adds its login/bootstrap schemas here).
const SCHEMAS = ['ClientConfig', 'Problem'];
// specs/examples documents the game never receives: CMS-only curriculum, server-side chat policy and the
// sensitive-term catalog (never sent to clients, AGENTS R15).
const NOT_FOR_CLIENT = [/^curriculum\./, /^chat-policy\./, /^sensitive-term-catalog\./];

const spec = yaml.load(fs.readFileSync(path.join(root, SPEC), 'utf8'));
const components = spec.components?.schemas ?? {};

function deref(schema, seen = new Set()) {
  if (schema && schema.$ref) {
    const name = schema.$ref.split('/').pop();
    if (seen.has(name)) return {};
    seen.add(name);
    return deref(components[name], seen);
  }
  return schema ?? {};
}

const FORMAT_SAMPLES = {
  uri: 'https://example.invalid/resource',
  uuid: '00000000-0000-4000-8000-000000000001',
  'date-time': '2026-11-15T07:30:00Z',
  date: '2026-11-15',
  email: 'student@example.invalid',
};

function sample(schemaIn, depth = 0, seen = new Set()) {
  const schema = deref(schemaIn, seen);
  if (schema.example !== undefined) return schema.example;
  if (Array.isArray(schema.examples) && schema.examples.length) return schema.examples[0];
  if (schema.default !== undefined) return schema.default;
  if (schema.allOf) return Object.assign({}, ...schema.allOf.map((s) => sample(s, depth + 1, new Set(seen))));
  if (schema.anyOf) return sample(schema.anyOf[0], depth + 1, new Set(seen));
  if (schema.oneOf) return sample(schema.oneOf[0], depth + 1, new Set(seen));
  if (schema.enum) return schema.enum[0];
  const type = Array.isArray(schema.type) ? schema.type.find((t) => t !== 'null') : schema.type;
  switch (type ?? (schema.properties ? 'object' : 'string')) {
    case 'object': {
      const out = {};
      if (depth > 6) return out;
      for (const [k, v] of Object.entries(schema.properties ?? {})) out[k] = sample(v, depth + 1, new Set(seen));
      return out;
    }
    case 'array':
      return depth > 6 ? [] : [sample(schema.items ?? {}, depth + 1, new Set(seen))];
    case 'integer':
      return schema.minimum ?? 1;
    case 'number':
      return schema.minimum ?? 1.5;
    case 'boolean':
      return false;
    default:
      return FORMAT_SAMPLES[schema.format] ?? (schema.pattern ? 'SAMPLE1' : 'sample');
  }
}

function render() {
  const files = {};
  const manifest = {};
  for (const name of SCHEMAS) {
    if (!components[name]) throw new Error(`schema ${name} not found in ${SPEC}`);
    const rel = `openapi/${name}.json`;
    files[rel] = JSON.stringify(sample({ $ref: `#/components/schemas/${name}` }), null, 2) + '\n';
    manifest[rel] = { source: `${SPEC}#/components/schemas/${name}` };
  }
  for (const file of fs.readdirSync(path.join(root, EXAMPLES_DIR)).filter((f) => f.endsWith('.json') && !NOT_FOR_CLIENT.some((re) => re.test(f))).sort()) {
    const rel = `examples/${file}`;
    files[rel] = fs.readFileSync(path.join(root, EXAMPLES_DIR, file), 'utf8');
    manifest[rel] = { source: `${EXAMPLES_DIR}/${file}` };
  }
  for (const [rel, text] of Object.entries(files)) {
    manifest[rel].sha256 = crypto.createHash('sha256').update(text).digest('hex');
  }
  files['manifest.json'] = JSON.stringify({ generated_by: 'src/game/tools/gen_fixtures.mjs', fixtures: manifest }, null, 2) + '\n';
  return files;
}

const files = render();
if (process.argv.includes('--check')) {
  const stale = Object.entries(files).filter(([rel, text]) => {
    const p = path.join(OUT_DIR, rel);
    return !fs.existsSync(p) || fs.readFileSync(p, 'utf8') !== text;
  });
  if (stale.length) {
    for (const [rel] of stale) console.error(`gen_fixtures: stale ${rel} — run node src/game/tools/gen_fixtures.mjs`);
    process.exit(1);
  }
  console.log(`gen_fixtures: OK (${Object.keys(files).length} files)`);
} else {
  fs.rmSync(OUT_DIR, { recursive: true, force: true });
  for (const [rel, text] of Object.entries(files)) {
    fs.mkdirSync(path.dirname(path.join(OUT_DIR, rel)), { recursive: true });
    fs.writeFileSync(path.join(OUT_DIR, rel), text);
  }
  console.log(`gen_fixtures: wrote ${Object.keys(files).length} files to ${path.relative(root, OUT_DIR)}`);
}
