// Applies the node labels of an inventory (F130 T001): role, zone, name (+ storage) and availability for each node,
// matched by hostname. Idempotent (`docker node update --label-add` overwrites the same value).
// Usage: node scripts/deploy/label-nodes.mjs deploy/swarm/nodes/<env>.yaml [--dry-run]   (DOCKER_HOST = a manager)
import fs from 'node:fs';
import { spawnSync } from 'node:child_process';
import { yaml } from './lib-policy.mjs';

export function commands(inventory) {
  const out = [];
  for (const n of inventory.nodes ?? []) {
    for (const k of ['name', 'hostname', 'role', 'zone']) if (!n[k] || !/^[a-z0-9][a-z0-9.-]{0,62}$/.test(String(n[k]))) throw new Error(`node ${n.name ?? '?'}: invalid ${k}`);
    const args = ['node', 'update', '--label-add', `role=${n.role}`, '--label-add', `zone=${n.zone}`, '--label-add', `name=${n.name}`];
    if (n.storage) args.push('--label-add', `storage=${n.storage}`);
    args.push('--availability', n.availability ?? 'active', n.hostname);
    out.push(args);
  }
  return out;
}

if (process.argv[1]?.endsWith('label-nodes.mjs')) {
  const [file, flag] = process.argv.slice(2);
  if (!file) { console.error('usage: label-nodes.mjs <inventory.yaml> [--dry-run]'); process.exit(2); }
  const cmds = commands(yaml.load(fs.readFileSync(file, 'utf8')));
  let rc = 0;
  for (const args of cmds) {
    if (flag === '--dry-run') { console.log(`docker ${args.join(' ')}`); continue; }
    const r = spawnSync(process.env.DOCKER_BIN ?? 'docker', args, { stdio: ['ignore', 'ignore', 'inherit'] });
    if (r.status !== 0) { console.error(`label-nodes: failed for ${args.at(-1)}`); rc = 1; }
  }
  process.exit(rc);
}
