// F004 T032 — F004-AC-22 (AGENTS R3): routes.yaml and the shared Keycloak realm contain no URL,
// hostname, IP/CIDR or host:port literal; environment values come from ${VAR} (routes/env/<env>.env)
// or $(env:KC_…) (keycloak-config-cli). Comment lines are ignored.
//   node deploy/apisix/scripts/check-no-hardcode.mjs
import { readFileSync } from 'node:fs';
import { isMain } from './lib.mjs';

const RULES = [
  ['URL', /\b[a-z][a-z0-9+.-]*:\/\/[^\s"'$]/i],
  ['IP address', /\b\d{1,3}(\.\d{1,3}){3}(\/\d{1,2})?\b/],
  ['hostname', /\b[a-z0-9-]+(\.[a-z0-9-]+)*\.(vn|com|net|org|io|edu|example|local|internal|svc|cluster)\b/i],
  ['host:port', /\b(localhost|[a-z][a-z0-9-]*):\d{2,5}\b/i],
];

// strip removes variable references so ${EW_UPSTREAM_CORE_API} or $(env:KC_PORTAL_ROOT_URL) never match.
const strip = (line) => line.replace(/\$\{[A-Z0-9_]+\}/g, '').replace(/\$\(env:[A-Za-z0-9_]+(:[^)]*)?\)/g, '');

export function scan(name, text) {
  const p = [];
  text.split('\n').forEach((raw, i) => {
    if (/^\s*#/.test(raw)) return;
    const line = strip(raw);
    for (const [what, re] of RULES) {
      const m = re.exec(line);
      if (m) p.push(`${name}:${i + 1}: ${what} literal "${m[0]}" (use a variable)`);
    }
  });
  return p;
}

export const FILES = ['deploy/apisix/routes/routes.yaml', 'deploy/keycloak/realm-eduworld.json'];

if (isMain(import.meta.url)) {
  const root = new URL('../../../', import.meta.url);
  const problems = FILES.flatMap((f) => scan(f, readFileSync(new URL(f, root), 'utf8')));
  for (const x of problems) console.error(x);
  console.log(`check-no-hardcode: ${FILES.length} files, ${problems.length} problem(s)`);
  process.exit(problems.length ? 1 : 0);
}
