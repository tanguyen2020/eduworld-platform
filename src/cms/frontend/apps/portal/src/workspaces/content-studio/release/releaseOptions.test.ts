// F054 T014 (+ T015 / T018 pure parts): rollback / archive choices (F054-AC-05, AC-08), read-only versions (AC-02),
// ICT ↔ UTC and the earliest scheduled publish (AC-04), diff groups with answers hidden without review (AC-03).
import { describe, expect, it } from 'vitest';
import { canArchive, diffGroup, diffValueText, earliestPublishAt, formatIct, groupDiff, ictToUtc, isReadOnlyVersion, isScheduleAllowed, rollbackTargets, utcToIct, type VersionSummary } from './releaseOptions';

const v = (version: number, status: VersionSummary['status']): VersionSummary => ({ version, status });

describe('releaseOptions', () => {
  const versions = [v(4, 'draft'), v(3, 'published'), v(2, 'published'), v(1, 'archived')];

  it('AC-05: rollback targets = published versions other than the current pointer, newest first', () => {
    expect(rollbackTargets(versions, 3).map((x) => x.version)).toEqual([2]);
    expect(rollbackTargets(versions, undefined).map((x) => x.version)).toEqual([3, 2]);
    expect(rollbackTargets([v(1, 'published')], 1)).toEqual([]);
  });

  it('AC-08: archive only a non-current, not archived version', () => {
    expect(canArchive(v(3, 'published'), 3)).toBe(false);
    expect(canArchive(v(2, 'published'), 3)).toBe(true);
    expect(canArchive(v(1, 'archived'), 3)).toBe(false);
    expect(canArchive(v(4, 'draft'), 3)).toBe(true);
  });

  it('AC-02: published and archived versions are read only', () => {
    expect(isReadOnlyVersion('published')).toBe(true);
    expect(isReadOnlyVersion('archived')).toBe(true);
    expect(isReadOnlyVersion('draft')).toBe(false);
    expect(isReadOnlyVersion('approved')).toBe(false);
  });

  it('AC-04: an ICT wall time is sent in UTC (UTC+7) and shown back in ICT', () => {
    const utc = ictToUtc({ year: 2026, month: 11, day: 21, hour: 7, minute: 30 });
    expect(utc.toISOString()).toBe('2026-11-21T00:30:00.000Z');
    expect(utcToIct(utc)).toEqual({ year: 2026, month: 11, day: 21, hour: 7, minute: 30 });
    expect(formatIct('2026-11-20T17:05:00Z')).toBe('21/11/2026 00:05');
    expect(formatIct(undefined)).toBe('');
  });

  it('AC-04: earliest publish_at = now + scheduled_publish_min_lead', () => {
    const now = new Date('2026-11-20T03:00:00Z');
    expect(earliestPublishAt(now, 5).toISOString()).toBe('2026-11-20T03:05:00.000Z');
    expect(isScheduleAllowed(new Date('2026-11-20T03:04:59Z'), now, 5)).toBe(false);
    expect(isScheduleAllowed(new Date('2026-11-20T03:05:00Z'), now, 5)).toBe(true);
  });

  it('AC-03: diff groups; answer changes dropped without learning_content.review', () => {
    expect(diffGroup('/stem/blocks/0/text')).toBe('text');
    expect(diffGroup('/answer_key/correct_option_id')).toBe('answer');
    expect(diffGroup('/render_payload/options/1/feedback')).toBe('answer');
    expect(diffGroup('/media_refs/0')).toBe('media');
    expect(diffGroup('/difficulty')).toBe('metadata');
    const changes = [
      { path: '/stem/blocks/0/text', op: 'changed' as const },
      { path: '/answer_key/correct_option_id', op: 'changed' as const },
      { path: '/media_refs/0', op: 'added' as const },
    ];
    expect(groupDiff(changes, false).answer).toEqual([]);
    expect(groupDiff(changes, true).answer).toHaveLength(1);
    expect(groupDiff(changes, false).text).toHaveLength(1);
    expect(diffValueText({ vi: 'x' })).toBe('{"vi":"x"}');
    expect(diffValueText('a'.repeat(500), 10)).toBe(`${'a'.repeat(10)}…`);
  });
});
