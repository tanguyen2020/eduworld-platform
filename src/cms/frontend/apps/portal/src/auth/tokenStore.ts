// tokenStore (F051 T017, F051-AC-02, ADR-0091 mục 4): the portal access token lives in process memory only — never
// localStorage, sessionStorage, IndexedDB or a cookie written by the portal. The refresh token is an HttpOnly cookie
// set by the backend that JavaScript cannot read. Nothing here is logged.

export interface AccessToken {
  token: string;
  /** epoch ms when the access token expires (`expires_in`). */
  expiresAt: number;
  /** epoch ms when the portal session ends at the latest (`refresh_expires_in`). */
  sessionExpiresAt: number;
  /** `auth_time` of the token (ISO-8601). */
  authTime: string;
  mfa: boolean;
}

export interface PortalTokenBody {
  access_token: string;
  expires_in: number;
  refresh_expires_in: number;
  auth_time: string;
  mfa: boolean;
}

export class TokenStore {
  private current: AccessToken | null = null;
  private readonly listeners = new Set<(t: AccessToken | null) => void>();

  constructor(private readonly now: () => number = Date.now) {}

  get(): AccessToken | null {
    return this.current;
  }

  /** Stores a `PortalTokenResponse` body (login or refresh). */
  setFromResponse(body: PortalTokenBody): AccessToken {
    const at = this.now();
    this.current = {
      token: body.access_token,
      expiresAt: at + body.expires_in * 1000,
      sessionExpiresAt: at + body.refresh_expires_in * 1000,
      authTime: body.auth_time,
      mfa: body.mfa,
    };
    this.emit();
    return this.current;
  }

  clear(): void {
    if (!this.current) return;
    this.current = null;
    this.emit();
  }

  subscribe(listener: (t: AccessToken | null) => void): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  private emit() {
    for (const l of this.listeners) l(this.current);
  }
}

/**
 * Subject (`sub`) of a JWT access token, read without verification — only to key per-viewer conveniences such as
 * the remembered tenant (data-model `ew.portal.<userSub>.*`). Returns null when the token is not a readable JWT.
 */
export function tokenSubject(token: string): string | null {
  const part = token.split('.')[1];
  if (!part) return null;
  try {
    const json = atob(part.replace(/-/g, '+').replace(/_/g, '/').padEnd(Math.ceil(part.length / 4) * 4, '='));
    const sub = (JSON.parse(json) as { sub?: unknown }).sub;
    return typeof sub === 'string' && /^[A-Za-z0-9-]{1,64}$/.test(sub) ? sub : null;
  } catch {
    return null;
  }
}
