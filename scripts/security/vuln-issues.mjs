// Daily rescan → GitHub issues (F132-AC-19, SECURITY_ARCHITECTURE §15): every CVE of severity CRITICAL that already has
// a fixed version, found by Trivy in an image running on staging/production, gets one open issue with the due date
// critical_vuln_fix (CONFIG_DEFAULTS: ≤ 72 hours from first detection). An issue already open for the same CVE and
// package is updated (new images listed), never duplicated. Uses the `gh` CLI with GITHUB_TOKEN (issues: write).
// Usage: node scripts/security/vuln-issues.mjs --env=staging [--dry-run] trivy-*.json
import fs from 'node:fs';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

export const FIX_HOURS = 72; // critical_vuln_fix (CONFIG_DEFAULTS, owner SECURITY_ARCHITECTURE §15)
export const LABELS = ['security', 'cve-critical'];

/** Critical vulnerabilities with a fix from parsed Trivy JSON reports: [{id, pkg, installed, fixed, image}]. */
export function criticalWithFix(reports) {
  const out = new Map();
  for (const r of reports) {
    const image = String(r.ArtifactName ?? '');
    for (const res of r.Results ?? []) {
      for (const v of res.Vulnerabilities ?? []) {
        if (v.Severity !== 'CRITICAL' || !v.FixedVersion) continue;
        const key = `${v.VulnerabilityID} ${v.PkgName}`;
        const cur = out.get(key) ?? { id: v.VulnerabilityID, pkg: v.PkgName, installed: v.InstalledVersion, fixed: v.FixedVersion, images: [] };
        if (image && !cur.images.includes(image)) cur.images.push(image);
        out.set(key, cur);
      }
    }
  }
  return [...out.values()].sort((a, b) => (a.id + a.pkg).localeCompare(b.id + b.pkg));
}

export function title(v) {
  return `[CVE critical] ${v.id} in ${v.pkg}`;
}

export function body(v, env, now) {
  const due = new Date(now.getTime() + FIX_HOURS * 3600 * 1000).toISOString();
  return [
    `Trivy rescan of the images running on **${env}** found ${v.id} (CRITICAL) in \`${v.pkg}\` ${v.installed}; fixed in ${v.fixed}.`,
    '',
    `**Due (critical_vuln_fix, ${FIX_HOURS} h):** ${due}`,
    '',
    'Images:',
    ...v.images.map((i) => `- \`${i}\``),
    '',
    'Process: SECURITY_ARCHITECTURE §15 (rebuild with the fixed version, release via backend/vX.Y.Z, redeploy). Opened by security-sweep.yml (F132-AC-19).',
  ].join('\n');
}

/** Opens or updates the issues. gh(args) runs the gh CLI and returns stdout. */
export function sync(vulns, env, now, gh) {
  const open = JSON.parse(gh(['issue', 'list', '--state', 'open', '--label', LABELS.join(','), '--limit', '500', '--json', 'number,title']) || '[]');
  const byTitle = new Map(open.map((i) => [i.title, i.number]));
  const actions = [];
  for (const v of vulns) {
    const t = title(v);
    if (byTitle.has(t)) {
      gh(['issue', 'comment', String(byTitle.get(t)), '--body', `Still present on ${env} (${now.toISOString()}):\n${v.images.map((i) => `- \`${i}\``).join('\n')}`]);
      actions.push({ action: 'updated', title: t });
    } else {
      gh(['issue', 'create', '--title', t, '--label', LABELS.join(','), '--body', body(v, env, now)]);
      actions.push({ action: 'created', title: t });
    }
  }
  return actions;
}

export function main(argv, { gh = (args) => execFileSync('gh', args, { encoding: 'utf8' }), now = new Date(), log = console.log } = {}) {
  let env = '';
  let dry = false;
  const files = [];
  for (const a of argv) {
    if (a.startsWith('--env=')) env = a.slice(6);
    else if (a === '--dry-run') dry = true;
    else files.push(a);
  }
  if (!['staging', 'production'].includes(env) || files.length === 0) {
    log('usage: vuln-issues.mjs --env=staging|production [--dry-run] trivy-*.json');
    return 2;
  }
  const vulns = criticalWithFix(files.map((f) => JSON.parse(fs.readFileSync(f, 'utf8'))));
  log(`vuln-issues: ${vulns.length} critical CVE(s) with a fix on ${env}`);
  if (dry) {
    for (const v of vulns) log(`  would open/update: ${title(v)}`);
    return 0;
  }
  for (const a of sync(vulns, env, now, gh)) log(`  ${a.action}: ${a.title}`);
  return 0;
}

if (process.argv[1] === fileURLToPath(import.meta.url)) process.exit(main(process.argv.slice(2)));
