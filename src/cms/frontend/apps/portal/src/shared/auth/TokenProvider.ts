// TokenProvider (F050 T030): source of the portal access token. F051 implements it (portal login through
// /auth/portal/*, ADR-0091): the access token lives in memory only; the refresh token is an HttpOnly cookie
// handled by the browser. apiClient never stores or logs tokens (F050-AC-17).
export interface TokenProvider {
  /** Current access token or null when signed out. */
  getAccessToken(): Promise<string | null>;
  /** Refreshes the access token; resolves false when the session cannot be renewed. */
  refresh(): Promise<boolean>;
}
