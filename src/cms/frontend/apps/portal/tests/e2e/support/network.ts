// Network log of a browser context for the Playwright E2E suites (real stack, OQ F051-9): every request the portal
// sends, with only the fields the suites assert on (path, query, X-Tenant-Id presence/value). Authorization and
// cookies are never kept (AGENTS R4, R14).
import type { BrowserContext } from '@playwright/test';

export interface LoggedRequest {
  method: string;
  /** Path of the URL (`/api/v1/...`, `/src/...` of the Vite dev server, `/assets/...` of a build). */
  path: string;
  /** Query string names (values dropped). */
  queryNames: string[];
  resourceType: string;
  /** Value of X-Tenant-Id, or null when the request did not send it. */
  tenantHeader: string | null;
}

export interface NetworkLog {
  all(): LoggedRequest[];
  /** API calls of the public/internal gateways (`/api/v1/…`, `/__internal/api/v1/…`). */
  api(): LoggedRequest[];
  clear(): void;
}

export function recordNetwork(context: BrowserContext): NetworkLog {
  const seen: LoggedRequest[] = [];
  context.on('request', (r) => {
    const u = new URL(r.url());
    const headers = r.headers();
    seen.push({
      method: r.method(),
      path: u.pathname,
      queryNames: [...new Set([...u.searchParams.keys()])],
      resourceType: r.resourceType(),
      tenantHeader: headers['x-tenant-id'] ?? null,
    });
  });
  return {
    all: () => [...seen],
    api: () => seen.filter((r) => /\/api\/v1\//.test(r.path)),
    clear: () => void seen.splice(0),
  };
}

/**
 * Requests that load the workspace C (school-lms) chunk: its entry module `src/workspaces/school-lms/index.tsx` on
 * the Vite dev server, or `assets/workspace-school-lms-<hash>.js` of a build (`check-chunks.mjs`). Shared modules
 * under `school-lms/dashboard/` that another workspace imports (platform-admin reports, F056 T040) are not the
 * workspace chunk: in a build they land in that workspace's chunk or a shared one.
 */
export const SCHOOL_LMS_CHUNK = /\/src\/workspaces\/school-lms\/index\.tsx$|\/assets\/workspace-school-lms-[^/]*\.js$/;

/** Operations of the School Learning Dashboard (catalog action `action.school_lms.school_dashboard.view` + F056 reports). */
export const DASHBOARD_API =
  /^\/(lms\/dashboards\/|lms\/schools\/[^/]+\/(grades\/[^/]+\/dashboard|reference-levels)|lms\/students\/[^/]+\/analytics|schools\/[^/]+\/academic-years|tenant\/analytics\/|lms\/reports|tenant\/reports)/;

/** API path without the gateway prefix (`/lms/assignments/…`). */
export const apiPath = (r: LoggedRequest) => r.path.replace(/^.*\/api\/v1/, '');
