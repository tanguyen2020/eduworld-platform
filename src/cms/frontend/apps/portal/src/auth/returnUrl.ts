// returnUrl (F051-AC-07, F051-AC-28, F051-AC-29): the URL to come back to after signing in. Only same-origin
// absolute paths are accepted (no `//host`, no scheme, no backslash) to avoid an open redirect; auth pages are
// never a return target.

const AUTH_PAGES = ['/login', '/forgot-password', '/reset-password', '/account-setup', '/select-tenant', '/no-access'];

const hasControlChar = (s: string) => [...s].some((c) => c.charCodeAt(0) < 0x20);

export function safeReturnUrl(value: string | null | undefined): string | null {
  if (!value || value.length > 2048) return null;
  if (!value.startsWith('/') || value.startsWith('//') || value.includes('\\') || hasControlChar(value)) return null;
  const path = value.split(/[?#]/)[0]!;
  if (AUTH_PAGES.some((p) => path === p || path.startsWith(`${p}/`))) return null;
  return value;
}

/** `/login?returnUrl=…` for the current location (or plain `/login`). */
export function loginPath(current?: string | null): string {
  const r = safeReturnUrl(current);
  return r && r !== '/' ? `/login?returnUrl=${encodeURIComponent(r)}` : '/login';
}

/** Return target of the current `?returnUrl=` query (default `/`). */
export function returnTarget(search: string): string {
  return safeReturnUrl(new URLSearchParams(search).get('returnUrl')) ?? '/';
}
