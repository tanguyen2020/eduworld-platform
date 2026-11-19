// useIdempotentMutation (F051 T050, AGENTS R10, F050 Clarification 2026-11-19): one Idempotency-Key per user
// intent. The mutation passes an intent id in the internal header `x-ew-intent` (stripped by the apiClient, which maps
// it to the same UUIDv7 key): retrying or clicking again with the same data reuses the key; changing the data starts a
// new intent; success forgets it. For the write screens of F052–F057.
import { useMutation, type UseMutationOptions } from '@tanstack/react-query';
import { useRef } from 'react';
import { INTENT_HEADER } from './middleware/idempotency';

export type IntentHeaders = { [INTENT_HEADER]: string };

/** Stable fingerprint of the mutation variables (object keys sorted). */
export function fingerprint(value: unknown): string {
  return JSON.stringify(value, (_k, v: unknown) =>
    v && typeof v === 'object' && !Array.isArray(v) ? Object.fromEntries(Object.entries(v as Record<string, unknown>).sort(([a], [b]) => a.localeCompare(b))) : v,
  );
}

/** Intent tracker (pure, testable): same data → same intent id; other data → a new one; `done` forgets. */
export class IntentTracker {
  private current: { fp: string; id: string } | null = null;

  constructor(private readonly newId: () => string = () => crypto.randomUUID()) {}

  intentFor(vars: unknown): string {
    const fp = fingerprint(vars);
    if (!this.current || this.current.fp !== fp) this.current = { fp, id: this.newId() };
    return this.current.id;
  }

  done(): void {
    this.current = null;
  }
}

export function useIdempotentMutation<TData, TVars>(
  mutationFn: (vars: TVars, headers: IntentHeaders) => Promise<TData>,
  options: Omit<UseMutationOptions<TData, unknown, TVars>, 'mutationFn'> = {},
) {
  const tracker = useRef<IntentTracker | null>(null);
  tracker.current ??= new IntentTracker();
  return useMutation<TData, unknown, TVars>({
    ...options,
    mutationFn: (vars) => mutationFn(vars, { [INTENT_HEADER]: tracker.current!.intentFor(vars) }),
    onSuccess: (...args) => {
      tracker.current!.done();
      return options.onSuccess?.(...args);
    },
  });
}
