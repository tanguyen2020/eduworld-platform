// SessionProvider (F051 T019, T022, T023, T041, T051): drives the session state machine with the portal runtime.
// - start: restore with refreshPortalToken unless the idle limit already passed (F051-AC-28/29);
// - after login/restore: listMyTenants (no X-Tenant-Id) → route (F051-AC-03); the working tenant is set before
//   any tenant-scoped query (F051-AC-05/06);
// - apiClient events: session expired (AC-07), staff only (AC-04), tenant mismatch/disabled (AC-21), feature
//   disabled (AC-37), permission denied (AC-12), VPN (AC-14);
// - sign-out clears token, query cache, working tenant and session storage even when the network fails (AC-08).
// Decisions about access stay in the backend; this only reflects its answers (AGENTS R1, R20).
import { App as AntApp } from 'antd';
import { createContext, useCallback, useContext, useEffect, useMemo, useReducer, useRef, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { usePortalRuntime, type PortalRuntime } from '../app/runtime';
import type { PortalTokenResponse } from '../auth/authApi';
import { setupTicketStore } from '../auth/memoryStores';
import { tokenSubject } from '../auth/tokenStore';
import { ApiError } from '../shared/api/middleware/problem';
import type { ApiEventDetail } from '../shared/api/events';
import { removeItem, STORAGE_KEYS } from '../shared/storage';
import { tenantFilterStore } from '../shell/tenantFilterStore';
import { initialSession, routeAfterLogin, scopeKey, sessionReducer, type MyTenant, type MyTenantsResponse, type SessionState } from './sessionMachine';
import { readRememberedTenant, readSessionTenant, writeRememberedTenant, writeSessionTenant } from './tenantSelection';

export type LoginNotice = 'sessionExpired' | 'setupTicketExpired' | 'passwordChanged' | 'passwordReset';

export interface SessionApi {
  state: SessionState;
  /** Key of the current scope for query keys (`platform` or tenant id). */
  scopeKey: string;
  /** `sub` of the access token (per-viewer storage keys), null when unknown. */
  userKey: string | null;
  notice: LoginNotice | null;
  setNotice(n: LoginNotice | null): void;
  vpnRequired: boolean;
  dismissVpn(): void;
  loginSucceeded(token: PortalTokenResponse): void;
  selectTenant(tenant: MyTenant, remember: boolean): void;
  /** Topbar switch (F051-AC-20): same token, new X-Tenant-Id, cache of the old tenant cleared. */
  switchTenant(tenant: MyTenant): void;
  retryScope(): void;
  logout(): Promise<void>;
  /** After the "session ended" dialog: back to /login keeping the URL. */
  acknowledgeExpired(): void;
}

const SessionContext = createContext<SessionApi | null>(null);

export function useSession(): SessionApi {
  const s = useContext(SessionContext);
  if (!s) throw new Error('SessionProvider missing');
  return s;
}

/** Clears every trace of the session in this tab (token, cache, tenant, session storage, timers). */
function clearLocalSession(rt: PortalRuntime) {
  rt.scheduler.stop();
  rt.idle.stop();
  rt.tokens.clear();
  rt.tenant.set(null);
  void rt.queryClient.cancelQueries();
  rt.queryClient.clear();
  writeSessionTenant(null);
  removeItem('session', STORAGE_KEYS.tenantFilter);
  tenantFilterStore.reset();
  setupTicketStore.clear();
}

export function SessionProvider({ children }: { children: ReactNode }) {
  const rt = usePortalRuntime();
  const { t } = useTranslation();
  const { notification } = AntApp.useApp();
  const [state, dispatch] = useReducer(sessionReducer, initialSession);
  const [notice, setNotice] = useState<LoginNotice | null>(null);
  const [vpnRequired, setVpnRequired] = useState(false);
  const [userKey, setUserKey] = useState<string | null>(null);
  const stateRef = useRef(state);
  stateRef.current = state;

  const expire = useCallback(() => {
    if (stateRef.current.status === 'unauthenticated' || stateRef.current.status === 'expired') return;
    clearLocalSession(rt);
    dispatch({ type: 'expired' });
  }, [rt]);

  const signOutLocally = useCallback(() => {
    clearLocalSession(rt);
    setUserKey(null);
    dispatch({ type: 'signed_out' });
  }, [rt]);

  const logout = useCallback(async () => {
    try {
      await rt.authApi.logout(rt.readCsrf());
    } finally {
      // the browser state is cleared even when the network fails (F051-AC-08)
      signOutLocally();
      rt.channel?.postMessage({ type: 'logout' });
    }
  }, [rt, signOutLocally]);

  // ---- start: restore the session (F051-AC-28) unless the idle limit passed (F051-AC-29)
  useEffect(() => {
    let cancelled = false;
    (async () => {
      if (rt.idle.isExpired()) {
        await rt.authApi.logout(rt.readCsrf()).catch(() => undefined);
        if (!cancelled) dispatch({ type: 'restore_failed' });
        return;
      }
      const r = await rt.scheduler.refreshDetailed();
      if (cancelled) return;
      if (r.ok) dispatch({ type: 'authenticated' });
      else dispatch({ type: 'restore_failed' });
    })();
    return () => {
      cancelled = true;
    };
  }, [rt]);

  // ---- resolve the scope after login/restore (F051-AC-03)
  useEffect(() => {
    if (state.status !== 'resolving_scope') return;
    let cancelled = false;
    const token = rt.tokens.get()?.token ?? null;
    const key = token ? tokenSubject(token) : null;
    setUserKey(key);
    (async () => {
      try {
        const res = await rt.api.call<MyTenantsResponse>(rt.api.client.GET('/me/tenants', {}));
        if (cancelled) return;
        const decision = routeAfterLogin(res, readRememberedTenant(key), readSessionTenant());
        if (decision.kind === 'tenant') {
          rt.tenant.set(decision.tenant.tenant_id);
          writeSessionTenant(decision.tenant.tenant_id);
        } else {
          rt.tenant.set(null);
          writeSessionTenant(null);
        }
        dispatch({ type: 'tenants_loaded', response: res, decision });
      } catch (e) {
        if (cancelled) return;
        if (e instanceof ApiError && e.code === 'PORTAL_ACCESS_STAFF_ONLY') return; // handled by the staff_only event
        dispatch({ type: 'tenants_failed' });
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [rt, state.status]);

  // ---- idle guard and proactive refresh failures
  useEffect(() => {
    const active = state.status === 'ready' || state.status === 'selecting_tenant' || state.status === 'resolving_scope';
    if (!active) return;
    rt.idle.setOnIdle(() => {
      void rt.authApi.logout(rt.readCsrf()).catch(() => undefined);
      setNotice('sessionExpired');
      signOutLocally();
      rt.channel?.postMessage({ type: 'logout' });
    });
    rt.scheduler.setOnExpired(() => expire());
    rt.idle.start();
    return () => rt.idle.stop();
  }, [rt, state.status, expire, signOutLocally]);

  // ---- apiClient events
  useEffect(() => {
    const ev = rt.api.events;
    const invalidatePermissions = () => void rt.queryClient.invalidateQueries({ queryKey: ['me', 'permissions'] });
    const offs = [
      ev.on('session_expired', () => expire()),
      ev.on('staff_only', () => {
        void rt.authApi.logout(rt.readCsrf()).catch(() => undefined);
        clearLocalSession(rt);
        dispatch({ type: 'staff_only' });
      }),
      ev.on('tenant_mismatch', () => {
        void rt.queryClient.cancelQueries();
        rt.queryClient.clear();
        rt.tenant.set(null);
        writeSessionTenant(null);
        notification.warning({ key: 'tenant-mismatch', message: t('error.TENANT_MISMATCH') });
        dispatch({ type: 'tenant_access_changed' });
      }),
      ev.on('feature_disabled', (d: ApiEventDetail) => {
        notification.warning({
          key: `feature-disabled-${d.featureKey ?? ''}`,
          message: t('shell.featureDisabled.title'),
          description: [t('shell.featureDisabled.description'), d.featureKey ? t('shell.featureDisabled.featureKey', { key: d.featureKey }) : '', d.traceId ? t('shell.problem.traceId', { traceId: d.traceId }) : '']
            .filter(Boolean)
            .join(' '),
        });
        invalidatePermissions();
      }),
      ev.on('permission_denied', () => invalidatePermissions()),
      ev.on('vpn_required', () => setVpnRequired(true)),
    ];
    return () => offs.forEach((off) => off());
  }, [rt, expire, notification, t]);

  // ---- other tabs signing out
  useEffect(() => {
    const ch = rt.channel;
    if (!ch) return;
    const onMessage = (e: MessageEvent) => {
      if ((e.data as { type?: string } | null)?.type === 'logout') signOutLocally();
    };
    ch.addEventListener('message', onMessage);
    return () => ch.removeEventListener('message', onMessage);
  }, [rt, signOutLocally]);

  const api = useMemo<SessionApi>(
    () => ({
      state,
      scopeKey: scopeKey(state.scope),
      userKey,
      notice,
      setNotice,
      vpnRequired,
      dismissVpn: () => setVpnRequired(false),
      loginSucceeded: (token) => {
        rt.tokens.setFromResponse(token);
        rt.scheduler.schedule();
        rt.idle.touch();
        setNotice(null);
        dispatch({ type: 'authenticated' });
      },
      selectTenant: (tenant, remember) => {
        rt.tenant.set(tenant.tenant_id);
        writeSessionTenant(tenant.tenant_id);
        writeRememberedTenant(userKey, remember ? tenant.tenant_id : null);
        dispatch({ type: 'tenant_selected', tenant });
      },
      switchTenant: (tenant) => {
        void rt.queryClient.cancelQueries();
        rt.queryClient.clear();
        rt.tenant.set(tenant.tenant_id);
        writeSessionTenant(tenant.tenant_id);
        setVpnRequired(false);
        dispatch({ type: 'tenant_selected', tenant });
      },
      retryScope: () => dispatch({ type: 'authenticated' }),
      logout,
      acknowledgeExpired: () => {
        setNotice('sessionExpired');
        dispatch({ type: 'restore_failed' });
      },
    }),
    [state, userKey, notice, vpnRequired, rt, logout],
  );

  return <SessionContext.Provider value={api}>{children}</SessionContext.Provider>;
}
