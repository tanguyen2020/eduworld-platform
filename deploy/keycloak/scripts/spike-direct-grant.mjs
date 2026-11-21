// Spike R5 (F004 T039, AC-29): what Keycloak returns for the direct grant flows of the realm.
import { passwordGrant, decodeJWT } from './kc.mjs';

const S = 'change-me-local', P = 'Demo@12345';
const show = (label, r) => {
  if (r.status === 200) {
    const t = decodeJWT(r.json.access_token);
    console.log(`${label}: 200 azp=${t.azp} amr=${JSON.stringify(t.amr)} acr=${t.acr} roles=${JSON.stringify(t.roles)} aud=${JSON.stringify(t.aud)} org=${t.organization ? 'yes' : 'no'} name=${t.given_name ?? t.name ?? '-'} email=${t.email ?? '-'}`);
  } else {
    console.log(`${label}: ${r.status} ${r.json.error} — ${r.json.error_description}`);
  }
};
show('portal tenant_staff', await passwordGrant('eduworld-portal', S, 'ad.truong.a', P));
show('portal tenant_staff by email', await passwordGrant('eduworld-portal', S, 'ad.truong.a@example.invalid', P));
show('portal platform_staff (password only, ADR-0094)', await passwordGrant('eduworld-portal', S, 'sa.admin', P));
show('portal safety_moderator (password only, ADR-0094)', await passwordGrant('eduworld-portal', S, 'sm.antoan', P));
show('portal required action UPDATE_PASSWORD', await passwordGrant('eduworld-portal', S, 'sa.moi', P));
show('portal student', await passwordGrant('eduworld-portal', S, 'hs.binh.3a', P));
show('portal wrong password', await passwordGrant('eduworld-portal', S, 'ad.truong.b', 'wrong'));
show('game student', await passwordGrant('eduworld-game-backend', S, 'hs.chi.3b', P));
show('game staff', await passwordGrant('eduworld-game-backend', S, 'ad.donvi', P));
show('game wrong password', await passwordGrant('eduworld-game-backend', S, 'hs.giang.6a', 'wrong'));
show('game wrong secret', await passwordGrant('eduworld-game-backend', 'nope', 'hs.hoa.6a', P));
show('svc-identity password grant', await passwordGrant('svc-identity', S, 'hs.khanh.8a', P));
show('admin-cli password grant on realm', await passwordGrant('admin-cli', undefined, 'hs.lan.9a', P));
