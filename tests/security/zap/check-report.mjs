// Gate of the ZAP scans (F132-AC-18, DEPLOYMENT §15): fails when a report has an alert of risk High (riskcode 3; ZAP
// has no Critical level) that is not marked false positive (confidence 0). Medium alerts are listed: each needs a plan
// (docs/10-security/pentest/README.md §4). Prints only rule ids, names, counts and URLs paths — never request or
// response evidence (it may contain tokens or synthetic personal data).
// Usage: node tests/security/zap/check-report.mjs <zap-report.json> [...more]   → exit 1 on High.
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';

const RISK = { 0: 'Informational', 1: 'Low', 2: 'Medium', 3: 'High' };

/** Returns {high: [...], medium: [...], total} for one parsed ZAP traditional-json report. */
export function evaluate(report) {
  const out = { high: [], medium: [], total: 0 };
  for (const site of report.site ?? []) {
    for (const a of site.alerts ?? []) {
      out.total++;
      const risk = Number(a.riskcode);
      if (String(a.confidence) === '0') continue; // false positive (marked in ZAP)
      const row = {
        site: String(site['@name'] ?? ''),
        pluginid: String(a.pluginid ?? ''),
        name: String(a.alert ?? a.name ?? ''),
        risk: RISK[risk] ?? String(risk),
        count: Number(a.count ?? (a.instances ?? []).length ?? 0),
        paths: [...new Set((a.instances ?? []).map((i) => pathOf(i.uri)).filter(Boolean))].slice(0, 5),
      };
      if (risk >= 3) out.high.push(row);
      else if (risk === 2) out.medium.push(row);
    }
  }
  return out;
}

function pathOf(uri) {
  try {
    return new URL(uri).pathname; // no query string: may carry identifiers
  } catch {
    return '';
  }
}

export function main(files, log = console.log) {
  if (files.length === 0) {
    log('usage: check-report.mjs <zap-report.json>...');
    return 2;
  }
  let high = 0;
  for (const f of files) {
    const r = evaluate(JSON.parse(fs.readFileSync(f, 'utf8')));
    high += r.high.length;
    log(`${f}: ${r.total} alert type(s), ${r.high.length} High, ${r.medium.length} Medium`);
    for (const a of r.high) log(`  HIGH   ${a.pluginid} ${a.name} ×${a.count} ${a.paths.join(' ')}`);
    for (const a of r.medium) log(`  MEDIUM ${a.pluginid} ${a.name} ×${a.count} (needs a plan)`);
  }
  return high > 0 ? 1 : 0;
}

if (process.argv[1] === fileURLToPath(import.meta.url)) process.exit(main(process.argv.slice(2)));
