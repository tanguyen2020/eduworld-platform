// Alert rules ↔ MONITORING §5 ↔ runbooks ↔ promtool tests (F131 T027, F131-AC-08, AC-12, AC-13).
//  - every alert of the §5 table has a rule of the same name in deploy/observability/prometheus/rules/, and every
//    alert rule is in the table (bijection; thresholds live in §5 only);
//  - every rule: severity critical|warning (one the table allows), annotations summary (≤ 200 chars), runbook_url =
//    docs/11-operations/runbooks/<AlertName>.md, source = a MONITORING section link; alert name ≤ 64 chars;
//  - every runbook_url points to an existing runbook with the sections of RUNBOOK_SECTIONS in order; no orphan runbook;
//  - every alert has a firing and a non-firing case in deploy/observability/prometheus/tests/*.test.yaml (Watchdog is
//    always firing: firing case only).
// Usage: node scripts/obs/check-alerts.mjs [--only=rules|runbooks] [--monitoring=<md>] [--rules=<dir>] [--runbooks=<dir>] [--tests=<dir>]
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  MONITORING, RULES_DIR, TESTS_DIR, RUNBOOKS_DIR, RUNBOOK_SECTIONS, yaml, readMonitoring, parseAlerts, loadRules, report,
} from './lib-monitoring-md.mjs';

const ALWAYS_FIRING = new Set(['Watchdog']);

export function checkAlerts({ monitoring = MONITORING, rulesDir = RULES_DIR, runbooksDir = RUNBOOKS_DIR, testsDir = TESTS_DIR, only = null } = {}) {
  const v = [];
  const table = parseAlerts(readMonitoring(monitoring));
  const byName = new Map(table.map((a) => [a.name, a]));
  const rules = loadRules(rulesDir).filter((r) => r.rule.alert);
  const ruleNames = new Set(rules.map((r) => r.rule.alert));
  const doRules = only !== 'runbooks';
  const doRunbooks = only !== 'rules';

  if (doRules) {
    for (const a of table) if (!ruleNames.has(a.name)) v.push({ file: monitoring, line: a.line, rule: 'missing-rule', message: `alert ${a.name} of MONITORING §5 has no rule` });
    for (const { file, rule } of rules) {
      const name = rule.alert;
      const add = (r, m) => v.push({ file, line: 1, rule: r, message: `${name}: ${m}` });
      const row = byName.get(name);
      if (!row) add('not-in-table', 'alert is not in the MONITORING §5 table (add the row with its threshold first)');
      if (name.length > 64) add('name-length', 'alert name longer than 64 characters');
      const sev = rule.labels?.severity;
      if (!['critical', 'warning'].includes(sev)) add('severity', `label severity must be critical or warning (got ${sev ?? 'none'})`);
      else if (row && row.severities.size && !row.severities.has(sev)) add('severity-table', `severity ${sev} is not allowed by MONITORING §5 (${[...row.severities].join('/')})`);
      const ann = rule.annotations ?? {};
      if (!ann.summary) add('summary', 'annotation summary is required');
      else if (String(ann.summary).length > 200) add('summary-length', 'summary longer than 200 characters');
      if (ann.runbook_url !== `docs/11-operations/runbooks/${name}.md`) add('runbook-url', `runbook_url must be docs/11-operations/runbooks/${name}.md`);
      if (!/^docs\/11-operations\/MONITORING\.md#\S+$/.test(ann.source ?? '')) add('source', 'annotation source must link the MONITORING section owning the threshold');
    }
    // promtool tests: firing + non-firing case per alert
    const cases = new Map();
    if (fs.existsSync(testsDir)) {
      for (const f of fs.readdirSync(testsDir).filter((x) => /\.test\.ya?ml$/.test(x))) {
        const doc = yaml.load(fs.readFileSync(path.join(testsDir, f), 'utf8')) ?? {};
        for (const t of doc.tests ?? []) for (const at of t.alert_rule_test ?? []) {
          const c = cases.get(at.alertname) ?? { fire: 0, quiet: 0 };
          if ((at.exp_alerts ?? []).length) c.fire++; else c.quiet++;
          cases.set(at.alertname, c);
        }
      }
    }
    for (const name of ruleNames) {
      const c = cases.get(name) ?? { fire: 0, quiet: 0 };
      const file = rules.find((r) => r.rule.alert === name).file;
      if (!c.fire) v.push({ file, line: 1, rule: 'test-firing', message: `${name}: no promtool case where the alert fires` });
      if (!c.quiet && !ALWAYS_FIRING.has(name)) v.push({ file, line: 1, rule: 'test-quiet', message: `${name}: no promtool case where the alert does not fire` });
    }
  }

  if (doRunbooks) {
    const names = new Set([...ruleNames, ...table.map((a) => a.name)]);
    for (const name of names) {
      const file = path.join(runbooksDir, `${name}.md`);
      if (!fs.existsSync(file)) { v.push({ file, line: 1, rule: 'runbook-missing', message: `runbook of ${name} is missing` }); continue; }
      const heads = fs.readFileSync(file, 'utf8').split(/\r?\n/).filter((l) => /^## /.test(l)).map((l) => l.slice(3).trim());
      let at = -1;
      for (const s of RUNBOOK_SECTIONS) {
        const i = heads.indexOf(s);
        if (i < 0) v.push({ file, line: 1, rule: 'runbook-section', message: `${name}: section "## ${s}" is missing` });
        else if (i < at) v.push({ file, line: 1, rule: 'runbook-order', message: `${name}: section "## ${s}" out of order` });
        else at = i;
      }
    }
    if (fs.existsSync(runbooksDir)) {
      for (const f of fs.readdirSync(runbooksDir).filter((x) => x.endsWith('.md') && x !== 'README.md')) {
        if (!names.has(f.replace(/\.md$/, ''))) v.push({ file: path.join(runbooksDir, f), line: 1, rule: 'runbook-orphan', message: `runbook ${f} matches no alert` });
      }
    }
  }
  return { violations: v, alerts: table.length, rules: rules.length };
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const opt = Object.fromEntries(process.argv.slice(2).map((a) => a.replace(/^--/, '').split('=')));
  const { violations, alerts, rules } = checkAlerts({
    only: opt.only ?? null, monitoring: opt.monitoring ?? MONITORING, rulesDir: opt.rules ?? RULES_DIR,
    runbooksDir: opt.runbooks ?? RUNBOOKS_DIR, testsDir: opt.tests ?? TESTS_DIR,
  });
  console.log(`check-alerts: ${alerts} alerts in MONITORING §5, ${rules} alert rules`);
  process.exit(report('check-alerts', violations));
}
