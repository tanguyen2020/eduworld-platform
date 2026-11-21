// apiClient events (F050 plan, extended by F051): the shell listens and reacts (back to login, tenant picker,
// VPN banner, staff-only page, feature-disabled notice, permission refresh). Payloads never carry tokens.
import type { ApiEventType } from './types';

export interface ApiEventDetail {
  operationId?: string;
  status?: number;
  /** Problem `code` that raised the event. */
  code?: string;
  traceId?: string;
  /** Problem `feature_key` of `403 FEATURE_DISABLED` (ADR-0090). */
  featureKey?: string;
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
