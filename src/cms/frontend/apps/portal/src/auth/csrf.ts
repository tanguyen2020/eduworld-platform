// CSRF double-submit (F051 T017, F051-AC-33, ADR-0091 mục 4): `refreshPortalToken` and `portalLogout` send the
// header `X-EW-CSRF` = value of the cookie `__Host-ew_portal_csrf` set by the backend (readable, not HttpOnly).
// The portal only READS this cookie; it never writes any cookie. The refresh token cookie is HttpOnly (invisible).

export const CSRF_COOKIE = '__Host-ew_portal_csrf';
export const CSRF_HEADER = 'X-EW-CSRF';

/** Current CSRF value, or null when the cookie is missing (blocked cookies, signed out). */
export function readCsrfToken(cookie: string = typeof document === 'undefined' ? '' : document.cookie): string | null {
  for (const part of cookie.split(';')) {
    const i = part.indexOf('=');
    if (i < 0) continue;
    if (part.slice(0, i).trim() === CSRF_COOKIE) {
      const v = part.slice(i + 1).trim();
      return /^[A-Za-z0-9_-]{43,64}$/.test(v) ? v : null;
    }
  }
  return null;
}
