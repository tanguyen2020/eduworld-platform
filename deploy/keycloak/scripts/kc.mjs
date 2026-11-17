// Small Keycloak Admin/OIDC helper for the realm scripts and tests (F004). Configuration from env:
// KC_URL (default http://localhost:8180), KC_ADMIN_USER, KC_ADMIN_PASSWORD (local default change-me-local).
export const KC_URL = process.env.KC_URL ?? 'http://localhost:8180';
export const REALM = process.env.KC_REALM ?? 'eduworld';

export async function adminToken() {
  const body = new URLSearchParams({
    grant_type: 'password', client_id: 'admin-cli',
    username: process.env.KC_ADMIN_USER ?? 'admin', password: process.env.KC_ADMIN_PASSWORD ?? 'change-me-local',
  });
  const r = await fetch(`${KC_URL}/realms/master/protocol/openid-connect/token`, { method: 'POST', body });
  if (!r.ok) throw new Error(`admin token: ${r.status} ${await r.text()}`);
  return (await r.json()).access_token;
}

export async function admin(path, { method = 'GET', body, token } = {}) {
  const t = token ?? await adminToken();
  const r = await fetch(`${KC_URL}/admin/realms/${REALM}${path}`, {
    method, headers: { authorization: `Bearer ${t}`, 'content-type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const text = await r.text();
  if (!r.ok && r.status !== 404 && r.status !== 409) throw new Error(`${method} ${path}: ${r.status} ${text}`);
  return { status: r.status, json: text ? safeJSON(text) : null, headers: r.headers };
}

function safeJSON(t) { try { return JSON.parse(t); } catch { return t; } }

// Resource-owner password grant against the realm (direct grant flows under test).
export async function passwordGrant(clientId, secret, username, password, extra = {}) {
  const body = new URLSearchParams({ grant_type: 'password', client_id: clientId, username, password, scope: 'openid', ...extra });
  if (secret !== undefined) body.set('client_secret', secret);
  const r = await fetch(`${KC_URL}/realms/${REALM}/protocol/openid-connect/token`, { method: 'POST', body });
  return { status: r.status, json: await r.json().catch(() => ({})) };
}

export function decodeJWT(t) {
  return JSON.parse(Buffer.from(t.split('.')[1], 'base64url').toString('utf8'));
}
