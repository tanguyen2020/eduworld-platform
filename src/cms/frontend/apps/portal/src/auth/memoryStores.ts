// In-memory holders (F051-AC-02, F051-AC-31, F051-AC-32): the setup ticket of `403 ACCOUNT_SETUP_REQUIRED` and the
// password reset token read from the email link. They live in module memory only — never in browser storage, router
// state (history.state survives reloads) or telemetry — and are cleared as soon as they are used.
import type { SetupRequired } from './authApi';

let setup: SetupRequired | null = null;
let resetToken: string | null = null;

export const setupTicketStore = {
  set(v: SetupRequired): void {
    setup = v;
  },
  get(): SetupRequired | null {
    return setup;
  },
  clear(): void {
    setup = null;
  },
};

export const resetTokenStore = {
  set(v: string): void {
    resetToken = v;
  },
  get(): string | null {
    return resetToken;
  },
  clear(): void {
    resetToken = null;
  },
};
