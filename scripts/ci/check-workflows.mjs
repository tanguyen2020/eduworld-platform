// Supply-chain and runner rules for GitHub Actions workflows (F001 T042, F001-AC-25, F001-AC-26, ADR-0088).
//   - every third-party `uses:` is pinned to a full 40-char commit SHA (local `./` actions are exempt);
//   - every workflow declares top-level `permissions` with `contents: read` (no write-all default);
//   - jobs exporting/signing iOS run on a macOS runner;
//   - jobs bound to Environment staging/production run only on [self-hosted, deploy] and never for fork PRs;
//   - secrets.APISIX_ADMIN_KEY only in step env of jobs bound to Environment staging/production (F130 T052).
// Usage: node scripts/ci/check-workflows.mjs [dir=.github/workflows]   → exit 1 on violations.
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const yaml = require('../node_modules/js-yaml');

const FULL_SHA = /^[0-9a-f]{40}$/;

/** Returns violations [{file, line, rule, message}] for one workflow source text. */
export function checkWorkflowText(file, text) {
  const lines = text.split(/\r?\n/);
  const out = [];
  const add = (line, rule, message) => out.push({ file, line, rule, message });
  const lineOf = (re, from = 0) => {
    for (let i = from; i < lines.length; i++) if (re.test(lines[i])) return i + 1;
    return 1;
  };

  // 1) pinned actions (line-based so the report points at the exact line)
  lines.forEach((l, i) => {
    const m = l.match(/^\s*-?\s*uses:\s*["']?([^"'\s#]+)/);
    if (!m) return;
    const ref = m[1];
    if (ref.startsWith('./') || ref.startsWith('docker://')) return;
    const at = ref.lastIndexOf('@');
    const pin = at >= 0 ? ref.slice(at + 1) : '';
    if (!FULL_SHA.test(pin)) add(i + 1, 'pin-sha', `"${ref}" must be pinned to a full commit SHA`);
  });

  let doc;
  try {
    doc = yaml.load(text) ?? {};
  } catch (e) {
    add(1, 'yaml', `invalid YAML: ${e.message}`);
    return out;
  }

  // 2) default permissions
  const perms = doc.permissions;
  if (perms === undefined) {
    add(1, 'permissions', 'top-level `permissions` is required (contents: read)');
  } else if (typeof perms === 'string' || perms === null || perms.contents !== 'read') {
    add(lineOf(/^permissions:/), 'permissions', 'top-level permissions must be a map with `contents: read`');
  }

  if (JSON.stringify(doc.env ?? {}).includes('secrets.APISIX_ADMIN_KEY')) {
    add(lineOf(/^env:/), 'apisix-admin-key', 'APISIX_ADMIN_KEY must never be in the workflow-level env');
  }

  // 3) runners per job
  for (const [id, job] of Object.entries(doc.jobs ?? {})) {
    const jobLine = lineOf(new RegExp(`^\\s{2}${id.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}:\\s*$`));
    const runsOn = [].concat(job['runs-on'] ?? []).map(String);
    const matrixTargets = [].concat(job.strategy?.matrix?.target ?? []).map(String);
    const isIOS = /ios/i.test(id) || /ios/i.test(job.name ?? '') || matrixTargets.some((t) => /ios/i.test(t));
    if (isIOS && !runsOn.some((r) => /^macos/.test(r) || r.includes('matrix.'))) {
      add(jobLine, 'ios-runner', `job "${id}" builds/signs iOS and must run on macos-*`);
    }

    const env = typeof job.environment === 'string' ? job.environment : job.environment?.name;

    // 4) APISIX admin key (F130 T052, DEPLOYMENT §13): only in jobs bound to Environment staging/production (or the
    //    environment input of a manual deploy workflow), and only in the env of the step that needs it.
    if (JSON.stringify(job).includes('secrets.APISIX_ADMIN_KEY')) {
      if (!env || !/^(staging|production)$|inputs\.environment/.test(env)) {
        add(jobLine, 'apisix-admin-key', `job "${id}" reads APISIX_ADMIN_KEY without Environment staging/production`);
      }
      if (JSON.stringify(job.env ?? {}).includes('secrets.APISIX_ADMIN_KEY')) {
        add(jobLine, 'apisix-admin-key', `job "${id}": APISIX_ADMIN_KEY only in the env of the step that uses it, not the job env`);
      }
    }
    if (env && /^(staging|production)$/.test(env)) {
      const selfDeploy = runsOn.includes('self-hosted') && runsOn.includes('deploy');
      if (!selfDeploy) {
        add(jobLine, 'deploy-runner', `job "${id}" uses environment ${env} and must run on [self-hosted, deploy]`);
      } else if (!/head\.repo\.fork\s*!=\s*true|head\.repo\.fork\s*==\s*false/.test(String(job.if ?? ''))) {
        add(jobLine, 'deploy-fork-guard', `job "${id}" must not run for pull requests from forks (if: …head.repo.fork != true)`);
      }
    }
  }
  return out;
}

export function checkDir(dir) {
  const files = fs.existsSync(dir) ? fs.readdirSync(dir).filter((f) => /\.ya?ml$/.test(f)) : [];
  return files.flatMap((f) => checkWorkflowText(path.join(dir, f), fs.readFileSync(path.join(dir, f), 'utf8')));
}

if (process.argv[1] && fileURLToPath(import.meta.url) === path.resolve(process.argv[1])) {
  const dir = process.argv[2] ?? '.github/workflows';
  const violations = checkDir(dir);
  for (const v of violations) console.error(`${v.file}:${v.line}: [${v.rule}] ${v.message}`);
  const n = fs.existsSync(dir) ? fs.readdirSync(dir).filter((f) => /\.ya?ml$/.test(f)).length : 0;
  console.log(`check-workflows: ${n} workflow, ${violations.length} lỗi`);
  process.exit(violations.length ? 1 : 0);
}
