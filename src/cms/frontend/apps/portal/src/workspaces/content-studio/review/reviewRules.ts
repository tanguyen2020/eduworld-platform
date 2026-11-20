// reviewRules (F054-AC-12, AC-13, AC-14, ADMIN_PORTAL §4 — pure display helpers): overdue warning of a review task
// (`due_at`, else submitted + `content_review_sla_days`), the bulk approve cap (`content_review_bulk_max`) and the
// per-field comments of a rejection (`field_comments[]` of transitionContentVersion, OQ F054-1 / F025 T046).
import type { components } from '../../../shared/api/schema.gen';
import { CONTENT_REVIEW_BULK_MAX, CONTENT_REVIEW_SLA_DAYS } from '../shared/config';
import { FIELDS } from '../shared/fieldSchemas';

export type ReviewTask = components['schemas']['ContentReviewTask'];

const DAY_MS = 24 * 60 * 60 * 1000;

export function dueAt(task: Pick<ReviewTask, 'due_at' | 'submitted_at'>): Date {
  return task.due_at ? new Date(task.due_at) : new Date(new Date(task.submitted_at).getTime() + CONTENT_REVIEW_SLA_DAYS * DAY_MS);
}

export function isOverdue(task: Pick<ReviewTask, 'due_at' | 'submitted_at' | 'status'>, now: Date): boolean {
  return (task.status ?? 'open') === 'open' && dueAt(task).getTime() < now.getTime();
}

/** F054-AC-14: at most `content_review_bulk_max` items per bulk approval. */
export function bulkAllowed(count: number): boolean {
  return count > 0 && count <= CONTENT_REVIEW_BULK_MAX;
}

export type FieldComment = components['schemas']['ReviewFieldComment'];

/**
 * Field comments sent with a rejection (JSON pointer `path`, empty = the whole version): trimmed, rows without a
 * comment dropped, a path written without the leading "/" ("stem/vi") turned into a pointer. The server validates
 * again (422 VALIDATION_FAILED).
 */
export function toFieldComments(rows: readonly { path?: string; comment?: string }[]): FieldComment[] {
  return rows
    .map((r) => ({ path: (r.path ?? '').trim(), comment: (r.comment ?? '').trim() }))
    .filter((r) => r.comment)
    .map((r) => ({ path: (r.path && !r.path.startsWith('/') ? `/${r.path}` : r.path).slice(0, FIELDS.reviewFieldPath.maxLength), comment: r.comment }));
}

export const taskKey = (t: Pick<ReviewTask, 'content_type' | 'content_id' | 'version'>) => `${t.content_type}:${t.content_id}:${t.version}`;
