// MOCK (test only, AGENTS R17): in-memory stand-in of listSchoolClientInstallers (specs/openapi/eduworld-api.v1.yaml,
// F134 US3) and of the listTenantSchools pages of a `unit` tenant, plugged into MockPortalBackend as overrides for the
// "Tải bộ cài" screen of workspace C. It records requests and answers fixed kits (fake URLs and hashes, no real build);
// it can answer any Problem (503 UNAVAILABLE, 403 FEATURE_DISABLED, 404). It verifies NO signature, enforces NO
// authorization and is NOT a backend — the real rules live in module game (F134-AC-13…16).
import { ACCOUNTS, TENANTS, type MockAccount, type MockPortalBackend } from './mock-backend';
import { accountWith, IDS, SCHOOL_PERMS } from './school-lms-mock';

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });

export const INST = {
  schoolU1: '0193a4d2-2000-7000-8000-0000000000e1',
  schoolU2: '0193a4d2-2000-7000-8000-0000000000e2',
  sha: 'a3f1c2d4e5b60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90',
  sha2: 'b4f1c2d4e5b60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f91',
  url: 'https://cdn.example.test/school-kit/production/eduworld-1.4.0-g1_3.apk',
  url2: 'https://cdn.example.test/school-kit/production/eduworld-1.4.0-g4_5.apk',
  webUrl: 'https://play.example.test/',
} as const;

/** MOCK kit list of TH Demo A (grades 1–5): two Android kits. */
export function installerList(extra: Record<string, unknown> = {}) {
  return {
    channel: 'production',
    index_generated_at: '2026-11-26T02:00:00Z',
    web_client_url: INST.webUrl,
    data: [
      { kit_id: 'kit-android-1.4.0-g1_3', platform: 'android', client_version: '1.4.0', grade_bands: ['g1_3'], url: INST.url, sha256: INST.sha, size_bytes: 159_822_643, min_os: 'Android 8.0', released_at: '2026-11-25T08:00:00Z' },
      { kit_id: 'kit-android-1.4.0-g4_5', platform: 'android', client_version: '1.4.0', grade_bands: ['g4_5'], url: INST.url2, sha256: INST.sha2, size_bytes: 171_966_464, min_os: 'Android 8.0', released_at: '2026-11-25T08:00:00Z' },
    ],
    ...extra,
  };
}

export function installersState() {
  return {
    list: installerList() as Record<string, unknown>,
    /** Answer of every listSchoolClientInstallers call instead of the list (e.g. 503 UNAVAILABLE). */
    answer: null as (() => Response) | null,
  };
}

export type InstallersState = ReturnType<typeof installersState>;

const unitSchool = (id: string, vi: string) => ({ school_id: id, tenant_id: TENANTS.pgdDemo.tenant_id, code: id.slice(-2), name: { vi }, status: 'active', version: 1 });

export function installInstallersMock(backend: MockPortalBackend, state: InstallersState): void {
  backend.overrides.unshift((r) => {
    if (r.method === 'GET' && r.path === '/tenant/client-installers') {
      if (state.answer) return state.answer();
      return json(200, state.list);
    }
    if (r.method === 'GET' && r.path === '/tenant/schools' && r.headers['x-tenant-id'] === TENANTS.pgdDemo.tenant_id) {
      // two cursor pages: the school picker must walk every page
      const page2 = new URLSearchParams(r.search).get('cursor') === 'p2';
      return json(
        200,
        page2
          ? { items: [unitSchool(INST.schoolU2, 'TH Đơn vị 2')], page: { has_more: false, next_cursor: null, limit: 100 } }
          : { items: [unitSchool(INST.schoolU1, 'TH Đơn vị 1')], page: { has_more: true, next_cursor: 'p2', limit: 100 } },
      );
    }
    return undefined;
  });
}

/** MOCK accounts: Tenant Admin of the `unit` tenant pgd-demo; staff of TH Demo A without school.installer.read. */
export const INSTALLER_ACCOUNTS: Record<string, MockAccount> = {
  'ad.donvi': accountWith('ad.donvi', TENANTS.pgdDemo, SCHOOL_PERMS, '0193a4d2-1111-7000-8000-0000000000e3'),
  'qlcm.khong.cai': accountWith('qlcm.khong.cai', TENANTS.thDemoA, SCHOOL_PERMS.filter((p) => p !== 'school.installer.read'), '0193a4d2-1111-7000-8000-0000000000e4'),
};

export interface RenderInstallersOptions {
  as?: string;
  state?: InstallersState;
  disabledMenus?: string[];
  disabledActions?: string[];
  width?: number;
}

/** MOCK (test only): the whole portal at `url` with the F052 and installer MOCK handlers (default `ad.truong.a`). */
export async function renderInstallers(url: string, opts: RenderInstallersOptions = {}) {
  const { renderSchoolLms } = await import('./school-lms-harness');
  const name = opts.as ?? 'ad.truong.a';
  const account = ACCOUNTS[name] ? undefined : INSTALLER_ACCOUNTS[name];
  const state = opts.state ?? installersState();
  const r = await renderSchoolLms(url, {
    as: account ? 'ad.truong.a' : name,
    ...(account ? { account } : {}),
    disabledMenus: opts.disabledMenus,
    disabledActions: opts.disabledActions,
    width: opts.width,
    beforeRender: (backend) => installInstallersMock(backend, state),
  });
  return { ...r, installers: state };
}

export { IDS };
