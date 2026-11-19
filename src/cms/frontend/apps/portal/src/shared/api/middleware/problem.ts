// problem (ADR-0010, API_DESIGN §4, F050-AC-14): Problem Details → ApiError mapped by `code` (never `title`)
// to the translation key `error.<CODE>`, `errors[]` of VALIDATION_FAILED → per-field errors, unknown code →
// `error.generic`, `trace_id` kept for the error detail.
import type { FieldError } from '../types';

export interface ApiErrorInit {
  status: number;
  code: string;
  i18nKey: string;
  title?: string;
  detail?: string;
  traceId?: string;
  fieldErrors?: Record<string, FieldError>;
  retryAfterMs?: number;
  retryable?: boolean;
  operationId?: string;
}

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly i18nKey: string;
  readonly title?: string;
  readonly detail?: string;
  readonly traceId?: string;
  readonly fieldErrors: Record<string, FieldError>;
  readonly retryAfterMs?: number;
  readonly retryable: boolean;
  readonly operationId?: string;

  constructor(init: ApiErrorInit) {
    super(`${init.status} ${init.code}`);
    this.name = 'ApiError';
    this.status = init.status;
    this.code = init.code;
    this.i18nKey = init.i18nKey;
    this.title = init.title;
    this.detail = init.detail;
    this.traceId = init.traceId;
    this.fieldErrors = init.fieldErrors ?? {};
    this.retryAfterMs = init.retryAfterMs;
    this.retryable = init.retryable ?? false;
    this.operationId = init.operationId;
  }
}

export type HasTranslation = (key: string) => boolean;

export const GENERIC_KEY = 'error.generic';

export function i18nKeyFor(code: string, hasTranslation: HasTranslation): string {
  const key = `error.${code}`;
  return /^[A-Z][A-Z0-9_]*$/.test(code) && hasTranslation(key) ? key : GENERIC_KEY;
}

interface ProblemBody {
  code?: unknown;
  title?: unknown;
  detail?: unknown;
  trace_id?: unknown;
  retryable?: unknown;
  errors?: unknown;
}

const str = (v: unknown) => (typeof v === 'string' ? v : undefined);

/** Builds an ApiError from a status + parsed body (Problem or anything else). */
export function toApiError(
  status: number,
  body: unknown,
  hasTranslation: HasTranslation,
  extra: { retryAfterMs?: number | null; operationId?: string } = {},
): ApiError {
  const p: ProblemBody = body && typeof body === 'object' ? (body as ProblemBody) : {};
  const code = str(p.code) ?? `HTTP_${status}`;
  const fieldErrors: Record<string, FieldError> = {};
  // errors[] of VALIDATION_FAILED and of the content checks (MATH_RENDER_FAILED, AUDIO_REQUIRED, TRANSLATION_INCOMPLETE,
  // MEDIA_NOT_READY — field = JSON pointer, F053) are kept per field
  if (Array.isArray(p.errors)) {
    for (const e of p.errors as Array<Record<string, unknown>>) {
      const field = str(e?.field);
      const c = str(e?.code);
      if (field && c) fieldErrors[field] = { code: c, ...(str(e.message) ? { message: str(e.message) } : {}) };
    }
  }
  return new ApiError({
    status,
    code,
    i18nKey: i18nKeyFor(code, hasTranslation),
    title: str(p.title),
    detail: str(p.detail),
    traceId: str(p.trace_id),
    fieldErrors,
    retryAfterMs: extra.retryAfterMs ?? undefined,
    retryable: p.retryable === true,
    operationId: extra.operationId,
  });
}

/** Reads the `code` of a Problem response without consuming the original body. */
export async function peekProblemCode(res: Response): Promise<string | undefined> {
  return (await peekProblem(res)).code;
}

/** Reads `code`, `trace_id` and `feature_key` (ADR-0090) of a Problem response without consuming the body. */
export async function peekProblem(res: Response): Promise<{ code?: string; traceId?: string; featureKey?: string }> {
  try {
    const body = (await res.clone().json()) as ProblemBody & { feature_key?: unknown };
    return { code: str(body?.code), traceId: str(body?.trace_id), featureKey: str(body?.feature_key) };
  } catch {
    return {};
  }
}
