// apiClient events (F050 plan): session_expired, tenant_mismatch, mfa_required, vpn_required. The shell (F051)
// listens and reacts (back to login, tenant picker, MFA prompt, VPN banner). Payloads never carry tokens.
import type { ApiEventType } from './types';

export interface ApiEventDetail {
  operationId?: string;
  status?: number;
  traceId?: string;
}

export class ApiEvents {
  private readonly target = new EventTarget();

  emit(type: ApiEventType, detail: ApiEventDetail = {}): void {
    this.target.dispatchEvent(new CustomEvent<ApiEventDetail>(type, { detail }));
  }

  /** Subscribes; returns the unsubscribe function. */
  on(type: ApiEventType, handler: (detail: ApiEventDetail) => void): () => void {
    const listener = (e: Event) => handler((e as CustomEvent<ApiEventDetail>).detail);
    this.target.addEventListener(type, listener);
    return () => this.target.removeEventListener(type, listener);
  }
}
