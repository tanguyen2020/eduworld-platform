// reviewRules (F054-AC-12, AC-13, AC-14, ADMIN_PORTAL §4 — pure display helpers): overdue warning of a review task
// (`due_at`, else submitted + `content_review_sla_days`), the bulk approve cap (`content_review_bulk_max`) and the
// rejection note with per-field comments. [Proposed] until `transitionContentVersion` has `field_comments[]` (Open
// question F054-2), field comments are written into `note` as "[path] comment" lines (≤ 2000 characters).
import type { components } from '../../../shared/api/schema.gen';
import { CONTENT_REVIEW_BULK_MAX, CONTENT_REVIEW_SLA_DAYS, LIMITS } from '../shared/config';

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

export interface FieldComment {
  path: string;
  comment: string;
}

/** Rejection note: the reason, then one "[path] comment" line per field note. */
export function composeReviewNote(reason: string, comments: readonly FieldComment[]): string {
  const lines = comments.filter((c) => c.path.trim() && c.comment.trim()).map((c) => `[${c.path.trim().slice(0, LIMITS.fieldPath)}] ${c.comment.trim()}`);
  return [reason.trim(), ...lines].join('\n');
}

export const noteFits = (note: string) => note.length <= LIMITS.transitionNote;

export const taskKey = (t: Pick<ReviewTask, 'content_type' | 'content_id' | 'version'>) => `${t.content_type}:${t.content_id}:${t.version}`;
