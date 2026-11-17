// Generates realm-eduworld.dev.json (local only, F004 T013/T043): Organizations and the demo accounts of
// LOCAL_DEVELOPMENT §5. Fake data only. Run: node deploy/keycloak/scripts/gen-dev-realm.mjs
import { writeFileSync } from 'node:fs';
import { DEMO_PASSWORD, SA_ADMIN_TOTP_SECRET } from './dev-constants.mjs';

const PASSWORD = DEMO_PASSWORD;

const orgs = [
  { alias: 'th-demo-a', name: 'TH Demo A' },
  { alias: 'pgd-demo', name: 'PGD Demo' },
  { alias: 'common', name: 'EduWorld chung' },
  { alias: 'qa-probe', name: 'QA probe' },
];
const students = {
  'th-demo-a': ['hs.an.3a', 'hs.binh.3a', 'hs.chi.3b', 'hs.trung.a1', 'hs.trung.a2', 'hs.locked', 'hs.lau.khongvao', 'hs.sapbidon'],
  'pgd-demo': ['hs.dung.7a', 'hs.giang.6a', 'hs.hoa.6a', 'hs.khanh.8a', 'hs.lan.9a', 'hs.em.4a'],
  common: ['hs.tudo'],
  'qa-probe': ['hs.qa.probe'],
  '': ['hs.nosis'], // no organization: ACCOUNT_NOT_PROVISIONED (ADR-0063)
};
const tenantStaff = {
  'ad.truong.a': ['th-demo-a'], 'ad.truong.b': ['pgd-demo'], 'qlcm.truong.a': ['th-demo-a', 'pgd-demo'],
  'ad.donvi': ['pgd-demo'], 'bt.noidung.dv': ['pgd-demo'], 'bt.duyet.dv': ['pgd-demo'],
};
const platformStaff = ['cm.noidung', 'cm.duyet', 'gd.thietke', 'lo.vanhanh', 'tc.thidau', 'sa.admin', 'sa.admin2', 'sm.antoan', 'sm.truongca'];

const pwd = { type: 'password', value: PASSWORD, temporary: false };
const users = [];
const members = Object.fromEntries(orgs.map((o) => [o.alias, []]));
for (const [org, names] of Object.entries(students)) {
  for (const u of names) {
    users.push({ username: u, enabled: true, emailVerified: false, realmRoles: ['student'], credentials: [pwd] });
    if (org) members[org].push(u);
  }
}
for (const [u, list] of Object.entries(tenantStaff)) {
  users.push({ username: u, email: `${u}@example.invalid`, enabled: true, emailVerified: true, realmRoles: ['tenant_staff'], credentials: [pwd] });
  for (const org of list) members[org].push(u);
}
for (const u of platformStaff) {
  const creds = [pwd];
  if (u === 'sa.admin') {
    creds.push({
      type: 'otp', userLabel: 'local-demo',
      secretData: JSON.stringify({ value: SA_ADMIN_TOTP_SECRET }),
      credentialData: JSON.stringify({ subType: 'totp', digits: 6, counter: 0, period: 30, algorithm: 'HmacSHA1' }),
    });
  }
  users.push({ username: u, email: `${u}@example.invalid`, enabled: true, emailVerified: true, realmRoles: ['platform_staff'], credentials: creds });
}
// Spike R5 / F012: platform staff with a temporary password and no OTP yet (UPDATE_PASSWORD + CONFIGURE_TOTP).
users.push({ username: 'sa.moi', email: 'sa.moi@example.invalid', enabled: true, emailVerified: true, realmRoles: ['platform_staff'],
  requiredActions: ['UPDATE_PASSWORD', 'CONFIGURE_TOTP'], credentials: [{ type: 'password', value: PASSWORD, temporary: true }] });

const realm = {
  realm: 'eduworld',
  users,
  organizations: orgs.map((o) => ({
    alias: o.alias, name: o.name, enabled: true,
    domains: [{ name: `${o.alias}.example.invalid`, verified: false }],
    members: members[o.alias].map((username) => ({ username, membershipType: 'MANAGED' })),
  })),
};
writeFileSync(new URL('../realm-eduworld.dev.json', import.meta.url), `${JSON.stringify(realm, null, 2)}\n`);
console.log(`realm-eduworld.dev.json: ${users.length} users, ${orgs.length} organizations`);
