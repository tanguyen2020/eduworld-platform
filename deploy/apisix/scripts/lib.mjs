// Shared helpers of the APISIX tooling (F004): YAML from scripts/node_modules, env files, routes.
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { pathToFileURL } from 'node:url';

const require = createRequire(new URL('../../../scripts/package.json', import.meta.url));
export const yaml = require('js-yaml');
export const ROOT = new URL('../', import.meta.url);

export function loadEnv(name) {
  const env = {};
  for (const line of readFileSync(new URL(`routes/env/${name}`, ROOT), 'utf8').split('\n')) {
    const m = /^([A-Z0-9_]+)=(.*)$/.exec(line.trim());
    if (m) env[m[1]] = m[2];
  }
  return env;
}

// substitute replaces ${VAR} in the YAML text (comment lines excluded) before parsing; an unknown
// variable is an error.
export function substitute(text, env) {
  return text.split('\n').map((line) => (/^\s*#/.test(line) ? line : line.replace(/\$\{([A-Z0-9_]+)\}/g, (_, k) => {
    if (!(k in env)) throw new Error(`routes.yaml: variable ${k} not set`);
    return env[k];
  }))).join('\n');
}

export function loadRoutes(env, file = 'routes/routes.yaml') {
  return yaml.load(substitute(readFileSync(new URL(file, ROOT), 'utf8'), env));
}

// isMain reports whether the module at url is the script node was started with (false under node -e / tests).
export const isMain = (url) => Boolean(process.argv[1]) && url === pathToFileURL(process.argv[1]).href;
