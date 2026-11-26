// Contract fixtures for the game client (F060 T041, F060-AC-22, GODOT_ARCHITECTURE §12).
// Generates src/game/tests/fixtures/contract/ from the single sources of truth:
//   - openapi/<Schema>.json: the `example` of each component schema the client parses, or a deterministic
//     sample built from the schema when the spec has no example (required + optional fields, first enum value);
//   - examples/<file>.json: copies of specs/examples/*.json (content documents the client will load);
//   - asyncapi/<type>.<name>.json: the `examples` of the realtime messages the client handles (F092 T005), each
//     validated against its message payload schema (ajv) — a wrong example fails generation;
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
// Component schemas the client parses: F060 frame + F061 login, refresh, tenant choice, game session, takeover;
// F062 school assignments (MyAssignment) and telemetry batches; F063 activity player (Attempt with the 11 question
// types, ActivityResponseResult, HintResult, AttemptReview — the OpenAPI examples); F070 player profile (GameProfile,
// CharacterCatalog, NicknameSuggestion, AvatarPreset); F064 exam zone (ExamSession, AttemptResult). F069 progression,
// economy and solo challenges (ProgressionStatus … ChallengeNextActivity).
const SCHEMAS = [
  'ClientConfig', 'Problem',
  'GameLoginResponse', 'GameTokenRefreshResponse', 'GameLoginProblem', 'ClientVersionProblem', 'GameSession',
  'ActiveSessionConflict', 'MyAssignment', 'TelemetryBatch',
  'Attempt', 'ActivityResponseResult', 'HintResult', 'AttemptReview',
  'GameProfile', 'CharacterCatalog', 'NicknameSuggestion', 'AvatarPreset',
  'ExamSession', 'AttemptResult',
  // F069: progression, wallet, inventory, shop, collections, achievements, solo challenges.
  'ProgressionStatus', 'XpLedgerEntry', 'ProgressionLevel', 'Wallet', 'CurrencyLedgerEntry', 'ItemLedgerEntry',
  'InventoryItem', 'ShopItem', 'PurchaseResult', 'AchievementProgress', 'CollectionProgress', 'CollectionDetail',
  'GameChallengeSummary', 'GameChallengeDetail', 'GameChallengeAttempt', 'ChallengeNextActivity',
  // F123: in-game notifications (inbox item).
  'Notification',
  // F102: party UI (party, invitation, eligible activity, session start, friend card of the invite picker).
  'Party', 'PartyInvitation', 'PartyEligibleActivity', 'PartySessionStart', 'PlayerCard',
];
// Response examples the client parses as a whole: [operationId, status] → openapi/<operationId>.<status>.json.
const RESPONSES = [['listMyAssignments', '200'], ['listNotifications', '200'],
  // F102: party lobby, invitations, eligible activities, friends of the invite picker, session start.
  ['getMyParty', '200'], ['listMyInvitations', '200'], ['listPartyEligibleActivities', '200'], ['listFriends', '200'],
  ['startPartySession', '201']];
// specs/examples documents the game never receives: CMS-only curriculum, server-side chat policy and the
// sensitive-term catalog (never sent to clients, AGENTS R15).
const NOT_FOR_CLIENT = [/^curriculum\./, /^chat-policy\./, /^sensitive-term-catalog\./];
// Realtime messages of the game client (F092): system, session, hub, presence; F123 notification; F102 party.
const ASYNCAPI = 'specs/asyncapi/eduworld-realtime.v1.yaml';
const ASYNC_PREFIXES = ['system.', 'session.', 'hub.', 'presence.', 'notification.', 'party.'];
const Ajv = require(path.join(root, 'scripts', 'node_modules', 'ajv'));
const addFormats = require(path.join(root, 'scripts', 'node_modules', 'ajv-formats'));

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

// A string matching `pattern` (codes such as CatalogCode `^[a-z0-9_]+$` get a valid lowercase code; F069).
function patternSample(pattern) {
  for (const candidate of ['sample1', 'SAMPLE1', 'sample', '1']) {
    try {
      if (new RegExp(pattern, 'u').test(candidate)) return candidate;
    } catch {
      return 'SAMPLE1';
    }
  }
  return 'SAMPLE1';
}

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
      return FORMAT_SAMPLES[schema.format] ?? (schema.pattern ? patternSample(schema.pattern) : 'sample');
  }
}

// AsyncAPI message examples → [rel, text, source]; every example must validate against its message payload.
function asyncExamples() {
  const doc = yaml.load(fs.readFileSync(path.join(root, ASYNCAPI), 'utf8'));
  const schemas = doc.components?.schemas ?? {};
  const toDefs = (v) => {
    if (Array.isArray(v)) return v.map(toDefs);
    if (v && typeof v === 'object') {
      const o = {};
      for (const [k, val] of Object.entries(v)) {
        if (k === 'examples' || k === 'example' || k.startsWith('x-')) continue;
        o[k] = k === '$ref' ? val.replace('#/components/schemas/', '#/$defs/') : toDefs(val);
      }
      return o;
    }
    return v;
  };
  const ajv = new Ajv({ allErrors: true, strict: false });
  addFormats(ajv);
  const out = [];
  for (const [key, msg] of Object.entries(doc.components?.messages ?? {}).sort(([a], [b]) => a.localeCompare(b))) {
    if (!msg?.name || !ASYNC_PREFIXES.some((p) => msg.name.startsWith(p)) || !Array.isArray(msg.examples)) continue;
    const validate = ajv.compile({ ...toDefs(msg.payload), $defs: toDefs(schemas) });
    for (const ex of msg.examples) {
      if (!validate(ex.payload)) {
        throw new Error(`${ASYNCAPI} ${msg.name} example ${ex.name}: ${ajv.errorsText(validate.errors)}`);
      }
      out.push([`asyncapi/${msg.name}.${ex.name}.json`, JSON.stringify(ex.payload, null, 2) + '\n',
        `${ASYNCAPI}#/components/messages/${key}/examples/${ex.name}`]);
    }
  }
  return out;
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
  for (const [operationId, status] of RESPONSES) {
    const op = Object.values(spec.paths ?? {}).flatMap((item) => Object.values(item)).find((o) => o?.operationId === operationId);
    const example = op?.responses?.[status]?.content?.['application/json']?.example;
    if (example === undefined) throw new Error(`no example for ${operationId} ${status} in ${SPEC}`);
    const rel = `openapi/${operationId}.${status}.json`;
    files[rel] = JSON.stringify(example, null, 2) + '\n';
    manifest[rel] = { source: `${SPEC}#${operationId}/responses/${status}/example` };
  }
  for (const file of fs.readdirSync(path.join(root, EXAMPLES_DIR)).filter((f) => f.endsWith('.json') && !NOT_FOR_CLIENT.some((re) => re.test(f))).sort()) {
    const rel = `examples/${file}`;
    files[rel] = fs.readFileSync(path.join(root, EXAMPLES_DIR, file), 'utf8');
    manifest[rel] = { source: `${EXAMPLES_DIR}/${file}` };
  }
  for (const [rel, text, source] of asyncExamples()) {
    files[rel] = text;
    manifest[rel] = { source };
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
