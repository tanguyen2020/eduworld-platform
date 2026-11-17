// Creates the Organizations of realm-eduworld.dev.json and their members through the Admin API
// (keycloak-config-cli 6.4 does not import organizations). Idempotent; local only (EW_ENV=local).
import { readFileSync } from 'node:fs';
import { admin, adminToken } from './kc.mjs';

const dev = JSON.parse(readFileSync(new URL('../realm-eduworld.dev.json', import.meta.url), 'utf8'));
const token = await adminToken();
const existing = (await admin('/organizations?max=1000', { token })).json ?? [];
let created = 0, added = 0;
for (const org of dev.organizations ?? []) {
  let o = existing.find((x) => x.alias === org.alias);
  if (!o) {
    const r = await admin('/organizations', { method: 'POST', token,
      body: { alias: org.alias, name: org.name, enabled: org.enabled, domains: org.domains } });
    if (r.status !== 201 && r.status !== 409) throw new Error(`create organization ${org.alias}: ${r.status} ${JSON.stringify(r.json)}`);
    o = ((await admin(`/organizations?search=${encodeURIComponent(org.name)}&exact=true`, { token })).json ?? []).find((x) => x.alias === org.alias);
    created++;
  }
  const current = new Set(((await admin(`/organizations/${o.id}/members?max=1000`, { token })).json ?? []).map((m) => m.username));
  for (const m of org.members ?? []) {
    if (current.has(m.username)) continue;
    const u = ((await admin(`/users?username=${encodeURIComponent(m.username)}&exact=true`, { token })).json ?? [])[0];
    if (!u) throw new Error(`member ${m.username} of ${org.alias} not found (import realm-eduworld.dev.json first)`);
    const r = await fetch(`${process.env.KC_URL ?? 'http://localhost:8180'}/admin/realms/eduworld/organizations/${o.id}/members`, {
      method: 'POST', headers: { authorization: `Bearer ${token}`, 'content-type': 'application/json' }, body: JSON.stringify(u.id) });
    if (r.status !== 201 && r.status !== 409) throw new Error(`add ${m.username} to ${org.alias}: ${r.status} ${await r.text()}`);
    added++;
  }
}
console.log(`organizations: ${created} created, ${added} members added`);
