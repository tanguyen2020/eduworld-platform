// releaseOptions (F054 plan "domain (client)", ADR-0043, CONTENT_MODEL §5–§6): pure display helpers of the release
// screens — which versions may be offered as rollback target / archived, read-only versions, the earliest scheduled
// publish time (shown in ICT, sent in UTC) and the grouping of a version diff. The server decides again every time.
import type { components } from '../../../shared/api/schema.gen';

export type VersionSummary = components['schemas']['ContentVersionSummary'];
export type DiffChange = components['schemas']['ContentVersionDiff']['changes'][number];

/** F054-AC-05: published versions of the same item other than the current pointer (newest first). */
export function rollbackTargets(versions: readonly VersionSummary[], current: number | undefined): VersionSummary[] {
  return versions.filter((v) => v.status === 'published' && v.version !== current).sort((a, b) => b.version - a.version);
}

/** F054-AC-08: any version not archived yet and not the current pointer. */
export function canArchive(v: Pick<VersionSummary, 'version' | 'status'>, current: number | undefined): boolean {
  return v.status !== 'archived' && v.version !== current;
}

/** F054-AC-02, AC-09: published and archived versions are read only (a new draft is made from them). */
export function isReadOnlyVersion(status: string): boolean {
  return status === 'published' || status === 'archived';
}

// ICT helpers shared with the Game Operations Console (F079): defined once in shared/release/ict.ts
export { ICT_OFFSET_MINUTES, ictToUtc, utcToIct, formatIct, earliestPublishAt, isScheduleAllowed } from '../../../shared/release/ict';
export type { WallTime } from '../../../shared/release/ict';

/** Diff groups shown by VersionDiffView (F054-AC-03). */
export type DiffGroup = 'text' | 'answer' | 'media' | 'metadata';

const ANSWER = /^\/(answer_key|scoring_rule|explanation|hints|accepted_answers)(\/|$)|\/(feedback|misconception_tag|correct_option_ids?|is_correct)(\/|$)/;
const MEDIA = /(^|\/)(media_refs|media_ref|audio_ref|image_media_ref|background_media_ref|math_assets)(\/|$)|\/blocks\/\d+\/(media_ref|asset_id)(\/|$)/;
const TEXT = /^\/(stem|render_payload|blocks|title|summary|description|instructions|definition|content|problem|steps|caption)(\/|$)/;

export function diffGroup(path: string): DiffGroup {
  if (ANSWER.test(path)) return 'answer';
  if (MEDIA.test(path)) return 'media';
  if (TEXT.test(path)) return 'text';
  return 'metadata';
}

/**
 * Groups the changes; answer changes are dropped without `learning_content.review` (the server never sends them
 * then — defence in depth, F054-AC-03, AGENTS R6).
 */
export function groupDiff(changes: readonly DiffChange[], canReview: boolean): Record<DiffGroup, DiffChange[]> {
  const out: Record<DiffGroup, DiffChange[]> = { text: [], answer: [], media: [], metadata: [] };
  for (const c of changes) {
    const g = diffGroup(c.path);
    if (g === 'answer' && !canReview) continue;
    out[g].push(c);
  }
  return out;
}

// shared with the Game Operations Console (F079): defined once in shared/release/DiffTable.tsx
export { diffValueText } from '../../../shared/release/DiffTable';
