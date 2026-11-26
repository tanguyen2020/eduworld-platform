// Swarm stack rules (F130-AC-01, AC-06, AC-07; DEPLOYMENT §4.3, §4.5, §14; policy deploy/swarm/policy.yaml).
// For every environment: the four stack files exist, app.stack.yml holds exactly the five backend Swarm services with
// the immutable ghcr.io image, every service has placement by node.labels.role, resources limits + reservations,
// update_config (start-first / failure_action rollback / monitor; stateful services stop-first), rollback_config, a
// healthcheck (backend: `<binary> healthcheck --endpoint=/livez`), the drain grace period, only the networks allowed by
// DEPLOYMENT §4.3; datastores never publish ports nor join `edge`; no image tagged latest or without a fixed tag.
// Usage: node scripts/deploy/check-stacks.mjs [envDir…]   (default deploy/swarm/{staging,production}) → exit 1 on violations.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { REPO_ROOT, yaml, loadPolicy, loadStacks, anyGlob, serviceLine, lineOf, readEnvFile, asList } from './lib-policy.mjs';

const durSeconds = (v) => {
  if (v == null) return 0;
  const m = String(v).match(/^(?:(\d+)m)?(?:(\d+)s)?$/);
  return m ? Number(m[1] ?? 0) * 60 + Number(m[2] ?? 0) : NaN;
};

/** Returns violations [{file, line, rule, message}] of one environment directory. */
export function checkEnv(envDir, policy = loadPolicy()) {
  const out = [];
  const stacks = loadStacks(envDir, policy);
  const backend = policy.backend_services ?? {};
  // Node inventory of the environment (deploy/swarm/nodes/<env>.yaml): pinned nodes must exist with the right role.
  const invFile = path.join(path.dirname(envDir), 'nodes', `${path.basename(envDir)}.yaml`);
  const inventory = fs.existsSync(invFile) ? (yaml.load(fs.readFileSync(invFile, 'utf8'))?.nodes ?? []) : null;
  if (!inventory) out.push({ file: invFile, line: 1, rule: 'inventory', message: 'node inventory deploy/swarm/nodes/<env>.yaml is missing (F130 T001)' });
  for (const st of stacks) {
    if (st.missing) { out.push({ file: st.file, line: 1, rule: 'stack-file', message: 'required stack file is missing (F130-AC-01)' }); continue; }
    const { file, text, doc } = st;
    const add = (line, rule, message) => out.push({ file, line, rule, message });
    const services = doc.services ?? {};
    const isApp = path.basename(file) === 'app.stack.yml';
    if (isApp) {
      const names = Object.keys(services).sort();
      const want = Object.keys(backend).sort();
      if (names.join(',') !== want.join(',')) add(1, 'backend-services', `app.stack.yml must contain exactly ${want.join(', ')} (got ${names.join(', ')})`);
    }
    for (const [name, raw] of Object.entries(services)) {
      const svc = raw ?? {};
      const at = serviceLine(text, name);
      // Line of re inside this service block only (keys inherited from an anchor point at the service line).
      const blockEnd = lineOf(text, /^ {2}\S|^\S/, at + 1, Infinity);
      const find = (re) => { const l = lineOf(text, re, at, at); return l < blockEnd ? l : at; };
      const isBackend = Boolean(backend[name]);
      const isData = anyGlob(policy.datastores, name);
      const deploy = svc.deploy ?? {};

      // image
      const image = String(svc.image ?? '');
      const tag = image.includes('@') ? 'digest' : image.slice(image.lastIndexOf('/') + 1).split(':')[1];
      if (!image) add(at, 'image', `${name}: image is required`);
      else if (!tag) add(find(/image:/), 'image-tag', `${name}: image ${image} has no explicit tag`);
      else if (/^latest$|\.x$|^\$\{[A-Z_]+:-latest\}$/.test(tag)) add(find(/image:/), 'image-latest', `${name}: image tag "${tag}" is not immutable`);
      if (isBackend) {
        const want = `ghcr.io/\${GHCR_ORG}/eduworld/${backend[name].image}:\${BACKEND_VERSION}`;
        if (image !== want) add(find(/image:/), 'backend-image', `${name}: image must be ${want}`);
      }

      // placement
      const constraints = (deploy.placement?.constraints ?? []).map(String);
      // mode global (F131): only per-node exporters/agents of policy global_services, which need no role constraint.
      const isGlobal = deploy.mode === 'global';
      const globalOk = isGlobal && anyGlob(policy.global_services ?? [], name);
      if (isGlobal && !globalOk) add(at, 'global-mode', `${name}: mode global is reserved to ${(policy.global_services ?? []).join(', ')}`);
      if (!globalOk && !constraints.some((c) => /^node\.labels\.role\s*==\s*\S+$/.test(c))) add(at, 'placement', `${name}: deploy.placement.constraints needs node.labels.role == <role>`);
      // Docker socket (F131): only the services of policy docker_socket, read-only; everything else uses an allowlist proxy.
      for (const vol of asList(svc.volumes)) {
        const spec = typeof vol === 'string' ? vol : `${vol?.source ?? ''}:${vol?.target ?? ''}${vol?.read_only ? ':ro' : ''}`;
        if (!/docker\.sock/.test(spec)) continue;
        if (!anyGlob(policy.docker_socket ?? [], name)) add(find(/docker\.sock/), 'docker-socket', `${name}: the Docker socket may only be mounted by ${(policy.docker_socket ?? []).join(', ') || 'nobody'}`);
        else if (!/:ro$/.test(spec)) add(find(/docker\.sock/), 'docker-socket', `${name}: the Docker socket must be mounted read-only`);
      }
      const role = constraints.map((c) => c.match(/^node\.labels\.role\s*==\s*(\S+)$/)?.[1]).find(Boolean);
      const pinned = constraints.map((c) => c.match(/^node\.labels\.name\s*==\s*(\S+)$/)?.[1]).find(Boolean);
      if (inventory && role && !inventory.some((n) => n.role === role)) add(at, 'placement-role', `${name}: no node with role ${role} in the inventory`);
      if (inventory && pinned) {
        const node = inventory.find((n) => n.name === pinned);
        if (!node) add(at, 'placement-node', `${name}: pinned to ${pinned}, which is not in the node inventory`);
        else if (role && node.role !== role) add(at, 'placement-node', `${name}: pinned to ${pinned} (role ${node.role}) but constrained to role ${role}`);
      }
      if (isData && !constraints.some((c) => /^node\.labels\.name\s*==/.test(c)) && !deploy.placement?.max_replicas_per_node) {
        add(at, 'placement-pin', `${name}: a stateful service must be pinned (node.labels.name) or spread with max_replicas_per_node`);
      }

      // resources
      const lim = deploy.resources?.limits, res = deploy.resources?.reservations;
      if (!lim?.cpus || !lim?.memory) add(at, 'resources', `${name}: deploy.resources.limits.cpus and .memory are required`);
      if (!res?.cpus || !res?.memory) add(at, 'resources', `${name}: deploy.resources.reservations.cpus and .memory are required`);

      // update / rollback
      const uc = deploy.update_config, rc = deploy.rollback_config;
      const stopFirstOk = isData || anyGlob(policy.stop_first_allowed, name);
      if (!uc) add(at, 'update-config', `${name}: deploy.update_config is required`);
      else {
        if (uc.order !== 'start-first' && !(stopFirstOk && uc.order === 'stop-first')) add(at, 'update-order', `${name}: update_config.order must be start-first`);
        if (uc.failure_action !== 'rollback' && !(isData && uc.failure_action === 'pause')) add(at, 'update-failure', `${name}: update_config.failure_action must be rollback`);
        if (!uc.monitor) add(at, 'update-monitor', `${name}: update_config.monitor is required`);
      }
      if (!rc) add(at, 'rollback-config', `${name}: deploy.rollback_config is required`);

      // healthcheck
      const hc = svc.healthcheck;
      if (!hc) add(at, 'healthcheck', `${name}: healthcheck is required`);
      else if (hc.disable === true && !anyGlob(policy.healthcheck_exempt, name)) add(at, 'healthcheck', `${name}: healthcheck cannot be disabled`);
      if (isBackend) {
        const bin = backend[name].image;
        const test = JSON.stringify(hc?.test ?? []);
        if (test !== JSON.stringify(['CMD', `/app/${bin}`, 'healthcheck', '--endpoint=/livez'])) {
          add(find(/healthcheck:/), 'healthcheck-livez', `${name}: Swarm healthcheck must be ["CMD", "/app/${bin}", "healthcheck", "--endpoint=/livez"] (DEPLOYMENT §14)`);
        }
        const grace = durSeconds(svc.stop_grace_period), min = durSeconds(backend[name].stop_grace_period_min);
        if (!(grace >= min)) add(at, 'stop-grace', `${name}: stop_grace_period must be ≥ ${backend[name].stop_grace_period_min} (drain)`);
        if (backend[name].profile && !JSON.stringify(svc.command ?? []).includes(`--profile=${backend[name].profile}`)) {
          add(at, 'worker-profile', `${name}: command must select --profile=${backend[name].profile}`);
        }
        const envFiles = asList(svc.env_file).map((f) => readEnvFile(path.join(path.dirname(file), f)));
        const admin = envFiles.flat().filter(Boolean).find((e) => e.key === 'EW_ADMIN_ADDR');
        if (admin?.value !== ':9090') add(at, 'admin-port', `${name}: EW_ADMIN_ADDR=:9090 is required in its env_file (one admin port, never published)`);
      }

      // ports
      if ((svc.ports ?? []).length > 0) {
        if (isData) add(find(/ports:/), 'datastore-ports', `${name}: a datastore never publishes a port (F130-AC-07)`);
        else if (!anyGlob(policy.may_publish, name)) add(find(/ports:/), 'ports', `${name}: only ${policy.may_publish.join(', ')} may publish ports`);
      }

      // networks
      const nets = asList(svc.networks);
      if (nets.length === 0) add(at, 'networks', `${name}: networks must be explicit`);
      for (const n of nets) {
        const def = policy.networks?.[n];
        if (!def) add(find(/networks:/), 'network-unknown', `${name}: network ${n} is not defined by DEPLOYMENT §4.3`);
        else if (!anyGlob(def.services, name)) add(find(/networks:/), 'network-matrix', `${name}: not allowed on network ${n} (DEPLOYMENT §4.3)`);
        if (n === 'edge' && isData) add(find(/networks:/), 'datastore-edge', `${name}: a datastore is never reachable from edge (F130-AC-07)`);
      }
    }
    // stack-level networks
    for (const [n, def] of Object.entries(doc.networks ?? {})) {
      const pol = policy.networks?.[n];
      const line = lineOf(text, new RegExp(`^  ${n}:`));
      if (!pol) { add(line, 'network-unknown', `network ${n} is not defined by DEPLOYMENT §4.3`); continue; }
      if (pol.stack_local) {
        if (def?.external) add(line, 'network-local', `network ${n} is stack-local`);
        if (pol.encrypted && String(def?.driver_opts?.encrypted) !== 'true') add(line, 'network-encrypted', `network ${n} must be encrypted`);
      } else if (def?.external !== true) add(line, 'network-external', `network ${n} must be external (created encrypted by deploy/swarm/networks.sh)`);
    }
    // configs exist and carry a content version in their name
    for (const [k, c] of Object.entries(doc.configs ?? {})) {
      const line = lineOf(text, new RegExp(`^  ${k}:`));
      if (!c?.file || !fs.existsSync(path.join(path.dirname(file), c.file))) add(line, 'config-file', `config ${k}: file ${c?.file} is missing`);
      if (!/\$\{CFG_[A-Z0-9_]+\}/.test(String(c?.name ?? ''))) add(line, 'config-version', `config ${k}: name must carry \${CFG_<FILE>} (immutable configs)`);
    }
  }
  return out;
}

export function main(argv = process.argv.slice(2)) {
  const policy = loadPolicy();
  const dirs = argv.length ? argv : policy.envs.map((e) => path.join(REPO_ROOT, 'deploy', 'swarm', e));
  const out = dirs.flatMap((d) => checkEnv(d, policy));
  for (const v of out) console.error(`${path.relative(process.cwd(), v.file)}:${v.line}: [${v.rule}] ${v.message}`);
  if (out.length) { console.error(`check-stacks: ${out.length} violation(s)`); return 1; }
  console.log(`check-stacks: ${dirs.length} environment(s) ok`);
  return 0;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) process.exit(main());
