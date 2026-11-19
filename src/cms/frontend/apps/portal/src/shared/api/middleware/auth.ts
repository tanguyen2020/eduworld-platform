// auth: Bearer token from TokenProvider; one shared refresh in flight (F050-AC-15). Never logs or stores tokens.
import type { TokenProvider } from '../../auth/TokenProvider';

export function applyAuth(headers: Headers, token: string | null): void {
  if (token) headers.set('Authorization', `Bearer ${token}`);
  else headers.delete('Authorization');
}

/** Single-flight refresh: concurrent 401 TOKEN_EXPIRED share one TokenProvider.refresh() call. */
export function createRefresher(provider: TokenProvider): () => Promise<boolean> {
  let inFlight: Promise<boolean> | null = null;
  return () => {
    inFlight ??= provider
      .refresh()
      .catch(() => false)
      .finally(() => {
        inFlight = null;
      });
    return inFlight;
  };
}
