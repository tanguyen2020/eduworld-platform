// VideoLessonEditor (F058 US3, F058-AC-15…AC-21, VIDEO_LEARNING §3–§7): Video Manager of `video_lesson` — the video
// (library or upload, status followed to ready, duration from the asset, read only), chapters on the timeline,
// WebVTT subtitles (MediaKind subtitle, exactly one default, `vi` required for a video with speech), embedded questions
// at a time (published questions, required / skip after wrong), segments and branch points of an interactive video,
// the seek policy and the completion rule prefilled from `video_completion`, a length warning by grade, and a preview
// (MP4 variant, chapters, subtitles, pause at each cue). Early checks only (videoRules); module learning decides.
import { DeleteOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Checkbox, Flex, Form, Input, InputNumber, Radio, Space, Switch, Table, Typography } from 'antd';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { MediaPicker } from '../../media/MediaPicker';
import { useMediaAsset } from '../../media/useMediaUpload';
import { QuestionPicker } from '../../question-sets/SelectionRuleEditor';
import type { Config } from '../activityModel';
import type { ActivityEditorProps, BlockerContext } from '../editorRegistry';
import { recommendedMinutes, videoIssues, type Branch, type Chapter, type Cue, type Segment, type Subtitle, type VideoContent } from '../validators/videoRules';
import { SearchSelect } from '@eduworld/ops-ui';

/** video_completion (CONFIG_DEFAULTS): watched ≥ 90 % + required cues answered. */
export const VIDEO_COMPLETION = { mode: 'min_watched_ratio', min_watched_ratio: 0.9, require_required_cues: true } as const;
const RATES = [0.75, 1, 1.25, 1.5];
const SEEKS = ['watched_only', 'backward_only', 'free'] as const;

export function videoConfig(): Config {
  return {
    content: { chapters: [], subtitles: [], embedded_questions: [] },
    interaction: { allow_seek: 'watched_only', playback_rates: [0.75, 1, 1.25] },
    evaluation: { strategies: ['completion', 'auto'], max_score: 10 },
    feedback: { timing: 'immediate' },
    completion_rule: { ...VIDEO_COMPLETION },
  };
}

const obj = (v: unknown): Record<string, unknown> => (v && typeof v === 'object' ? (v as Record<string, unknown>) : {});
const sec = (ms: number | undefined) => (ms === undefined ? undefined : Math.round(ms / 100) / 10);
const ms = (s: number | null) => Math.round((s ?? 0) * 1000);

/** Local blockers of Gửi duyệt (F058-AC-15…AC-19): the rules of videoRules and media not ready yet. */
export function videoBlockers(config: Config, ctx: BlockerContext) {
  const c = obj(config.content) as VideoContent;
  const out = videoIssues(c, { speech: !!ctx.flags.speech }).map((i) => ({ field: i.pointer, message: ctx.t(`content_studio.activity.${i.key}`, i.values) }));
  const assets = [c.video_asset_id, ...(c.segments ?? []).map((s) => s.video_asset_id), ...(c.subtitles ?? []).map((s) => s.subtitle_asset_id)].filter(Boolean) as string[];
  for (const a of assets) {
    const st = ctx.mediaStatus[a];
    if (st && st !== 'ready') out.push({ field: '/config/content/video_asset_id', message: ctx.t(st === 'rejected' ? 'error.MEDIA_NOT_READY' : 'error.VIDEO_NOT_READY') });
  }
  return out;
}

function VideoPreview({ assetId, content }: { assetId: string; content: VideoContent }) {
  const { t } = useTranslation();
  const asset = useMediaAsset(assetId);
  const ref = useRef<HTMLVideoElement>(null);
  const [cue, setCue] = useState<Cue | null>(null);
  const done = useRef(new Set<string>());
  const mp4 = asset.data?.variants?.find((v) => v.variant.startsWith('mp4'));
  if (!mp4) return <Typography.Text type="secondary">{t('content_studio.activity.video.previewUnavailable')}</Typography.Text>;
  return (
    <Flex vertical gap="small" data-testid="video-preview">
      <video
        ref={ref}
        controls
        src={mp4.url}
        aria-label={t('content_studio.activity.video.preview')}
        style={{ width: '100%' }}
        onTimeUpdate={(e) => {
          const at = e.currentTarget.currentTime * 1000;
          const next = (content.embedded_questions ?? []).find((q) => !done.current.has(q.cue_id) && q.at_ms <= at);
          if (next) {
            done.current.add(next.cue_id);
            e.currentTarget.pause();
            setCue(next);
          }
        }}
      >
        {(content.subtitles ?? []).map((s) => (
          <SubtitleTrack key={s.language} subtitle={s} />
        ))}
      </video>
      <Space wrap>
        {(content.chapters ?? []).map((c) => (
          <Button key={c.chapter_id} size="small" onClick={() => ref.current && (ref.current.currentTime = c.start_ms / 1000)}>
            {c.title?.vi ?? c.chapter_id}
          </Button>
        ))}
      </Space>
      {cue && <Alert type="info" showIcon data-testid="preview-cue" message={t('content_studio.activity.video.cueReached', { id: cue.cue_id })} action={<Button size="small" onClick={() => { setCue(null); void ref.current?.play?.(); }}>{t('content_studio.activity.video.continue')}</Button>} />}
    </Flex>
  );
}

function SubtitleTrack({ subtitle }: { subtitle: Subtitle }) {
  const asset = useMediaAsset(subtitle.subtitle_asset_id);
  const vtt = asset.data?.variants?.find((v) => v.variant === 'vtt');
  return vtt ? <track kind="subtitles" srcLang={subtitle.language} label={subtitle.language} src={vtt.url} default={subtitle.is_default} /> : null;
}

export function VideoLessonEditor({ config, onChange, readOnly, errors, gradeLevels, onMediaStatus, flags, onFlag }: ActivityEditorProps) {
  const { t } = useTranslation();
  const content = obj(config.content) as VideoContent;
  const interaction = obj(config.interaction);
  const completion = obj(config.completion_rule);
  const speech = !!flags.speech;
  const [picking, setPicking] = useState<number | null>(null);
  const video = useMediaAsset(content.video_asset_id);
  const set = (patch: Partial<VideoContent>) => onChange({ ...config, content: { ...content, ...patch } });
  const duration = video.data?.duration_ms ?? content.duration_ms;
  useEffect(() => {
    if (video.data?.duration_ms && video.data.duration_ms !== content.duration_ms) set({ duration_ms: video.data.duration_ms });
    // eslint-disable-next-line react-hooks/exhaustive-deps -- copy the probed duration once per asset
  }, [video.data?.duration_ms]);
  const issues = videoIssues({ ...content, duration_ms: duration }, { speech });
  const issueAt = (p: string) => issues.find((i) => i.pointer === `/config/content/${p}`) ?? null;
  const msg = (p: string) => {
    const i = issueAt(p);
    return errors[`/config/content/${p}`] ?? (i ? t(`content_studio.activity.${i.key}`, i.values) : undefined);
  };
  const recommended = recommendedMinutes(gradeLevels);
  const videoMsg = msg('video_asset_id');
  const subtitleMsg = msg('subtitles');
  const segmentMsg = msg('segments');
  const chapters = content.chapters ?? [];
  const cues = content.embedded_questions ?? [];
  const subs = content.subtitles ?? [];
  const segments = content.segments ?? [];
  const branches = content.branches ?? [];
  const upd = <T,>(list: T[], i: number, patch: Partial<T>) => list.map((x, k) => (k === i ? { ...x, ...patch } : x));

  return (
    <Flex vertical gap="middle" data-testid="video-lesson-editor">
      <Card size="small" title={t('content_studio.activity.video.title')}>
        <MediaPicker id="video_asset_id" kind="video" value={content.video_asset_id} disabled={readOnly} onChange={(v) => set({ video_asset_id: v, duration_ms: undefined })} onStatus={onMediaStatus} />
        <Typography.Paragraph data-testid="video-duration">
          {t('content_studio.activity.video.duration', { s: duration ? sec(duration) : '—' })}
        </Typography.Paragraph>
        {recommended && duration && duration > recommended * 60_000 ? <Alert type="warning" showIcon data-testid="video-length-warning" message={t('content_studio.activity.video.tooLong', { min: recommended })} /> : null}
        {videoMsg && <Typography.Text type="danger">{videoMsg}</Typography.Text>}
      </Card>

      <Card size="small" title={t('content_studio.activity.video.chapters')}>
        <Table<Chapter>
          size="small"
          pagination={false}
          rowKey={(_, i) => String(i)}
          dataSource={chapters}
          data-testid="video-chapters"
          columns={[
            { title: t('content_studio.activity.video.chapterId'), render: (_, c, i) => <Input aria-label={t('content_studio.activity.video.chapterIdOf', { n: i + 1 })} maxLength={64} disabled={readOnly} value={c.chapter_id} onChange={(e) => set({ chapters: upd(chapters, i, { chapter_id: e.target.value }) })} /> },
            { title: t('content_studio.activity.video.chapterTitle'), render: (_, c, i) => <Input aria-label={t('content_studio.activity.video.chapterTitleOf', { n: i + 1 })} maxLength={128} showCount disabled={readOnly} value={c.title?.vi ?? ''} onChange={(e) => set({ chapters: upd(chapters, i, { title: { ...c.title, vi: e.target.value } }) })} /> },
            { title: t('content_studio.activity.video.start'), render: (_, c, i) => <InputNumber aria-label={t('content_studio.activity.video.startOf', { n: i + 1 })} min={0} step={0.5} disabled={readOnly} value={sec(c.start_ms)} onChange={(v) => set({ chapters: upd(chapters, i, { start_ms: ms(v) }) })} /> },
            {
              title: t('content_studio.activity.video.end'),
              render: (_, c, i) => (
                <Space direction="vertical" size={0}>
                  <InputNumber aria-label={t('content_studio.activity.video.endOf', { n: i + 1 })} min={0} step={0.5} disabled={readOnly} status={msg(`chapters/${i}/end_ms`) || msg(`chapters/${i}/start_ms`) ? 'error' : undefined} value={sec(c.end_ms)} onChange={(v) => set({ chapters: upd(chapters, i, { end_ms: ms(v) }) })} />
                  {(msg(`chapters/${i}/end_ms`) ?? msg(`chapters/${i}/start_ms`)) && <Typography.Text type="danger">{msg(`chapters/${i}/end_ms`) ?? msg(`chapters/${i}/start_ms`)}</Typography.Text>}
                </Space>
              ),
            },
            { title: '', render: (_, _c, i) => !readOnly && <Button size="small" icon={<DeleteOutlined />} aria-label={t('content_studio.activity.video.removeChapter', { n: i + 1 })} onClick={() => set({ chapters: chapters.filter((_, k) => k !== i) })} /> },
          ]}
        />
        {!readOnly && <Button onClick={() => set({ chapters: [...chapters, { chapter_id: `ch${chapters.length + 1}`, title: {}, start_ms: chapters.at(-1)?.end_ms ?? 0, end_ms: (chapters.at(-1)?.end_ms ?? 0) + 60_000 }] })}>{t('content_studio.activity.video.addChapter')}</Button>}
      </Card>

      <Card size="small" title={t('content_studio.activity.video.subtitles')}>
        <Checkbox checked={speech} onChange={(e) => onFlag('speech', e.target.checked)}>
          {t('content_studio.activity.video.speech')}
        </Checkbox>
        <Flex vertical gap="small">
          {subs.map((s, i) => (
            <Flex key={i} gap="small" align="center" data-testid={`subtitle-${i}`}>
              <SearchSelect aria-label={t('content_studio.activity.video.subtitleLanguageOf', { n: i + 1 })} disabled={readOnly} value={s.language} style={{ width: 90 }} options={['vi', 'en'].map((l) => ({ value: l, label: l }))} onChange={(v: string) => set({ subtitles: upd(subs, i, { language: v }) })} />
              <MediaPicker id={`subtitle_${i}`} kind="subtitle" value={s.subtitle_asset_id} disabled={readOnly} onChange={(v) => set({ subtitles: upd(subs, i, { subtitle_asset_id: v }) })} onStatus={onMediaStatus} />
              <Radio aria-label={t('content_studio.activity.video.subtitleDefaultOf', { n: i + 1 })} disabled={readOnly} checked={!!s.is_default} onChange={() => set({ subtitles: subs.map((x, k) => ({ ...x, is_default: k === i })) })}>
                {t('content_studio.activity.video.subtitleDefault')}
              </Radio>
              {!readOnly && <Button size="small" icon={<DeleteOutlined />} aria-label={t('content_studio.activity.video.removeSubtitle', { n: i + 1 })} onClick={() => set({ subtitles: subs.filter((_, k) => k !== i) })} />}
            </Flex>
          ))}
          {subtitleMsg && <Typography.Text type="danger" data-testid="subtitle-issue">{subtitleMsg}</Typography.Text>}
          {!readOnly && <Button onClick={() => set({ subtitles: [...subs, { language: subs.some((s) => s.language === 'vi') ? 'en' : 'vi', is_default: subs.length === 0 }] })}>{t('content_studio.activity.video.addSubtitle')}</Button>}
        </Flex>
      </Card>

      <Card size="small" title={t('content_studio.activity.video.cues')}>
        <Table<Cue>
          size="small"
          pagination={false}
          rowKey={(_, i) => String(i)}
          dataSource={cues}
          data-testid="video-cues"
          columns={[
            { title: t('content_studio.activity.video.cueId'), render: (_, q, i) => <Input aria-label={t('content_studio.activity.video.cueIdOf', { n: i + 1 })} maxLength={64} disabled={readOnly} value={q.cue_id} status={msg(`embedded_questions/${i}/cue_id`) ? 'error' : undefined} onChange={(e) => set({ embedded_questions: upd(cues, i, { cue_id: e.target.value }) })} /> },
            {
              title: t('content_studio.activity.video.at'),
              render: (_, q, i) => (
                <Space direction="vertical" size={0}>
                  <InputNumber aria-label={t('content_studio.activity.video.atOf', { n: i + 1 })} min={0} step={0.5} disabled={readOnly} value={sec(q.at_ms)} status={msg(`embedded_questions/${i}/at_ms`) ? 'error' : undefined} onChange={(v) => set({ embedded_questions: upd(cues, i, { at_ms: ms(v) }) })} />
                  {msg(`embedded_questions/${i}/at_ms`) && <Typography.Text type="danger">{msg(`embedded_questions/${i}/at_ms`)}</Typography.Text>}
                </Space>
              ),
            },
            { title: t('content_studio.activity.video.cueQuestion'), render: (_, q, i) => <Button size="small" disabled={readOnly} onClick={() => setPicking(i)}>{q.question_version_id ? q.question_version_id.slice(-8) : t('content_studio.activity.source.add')}</Button> },
            { title: t('content_studio.activity.video.required'), render: (_, q, i) => <Switch aria-label={t('content_studio.activity.video.requiredOf', { n: i + 1 })} disabled={readOnly} checked={q.required ?? true} onChange={(v) => set({ embedded_questions: upd(cues, i, { required: v }) })} /> },
            { title: t('content_studio.activity.video.skipAfterWrong'), render: (_, q, i) => <Switch aria-label={t('content_studio.activity.video.skipOf', { n: i + 1 })} disabled={readOnly} checked={q.allow_skip_after_wrong ?? true} onChange={(v) => set({ embedded_questions: upd(cues, i, { allow_skip_after_wrong: v }) })} /> },
            { title: '', render: (_, _q, i) => !readOnly && <Button size="small" icon={<DeleteOutlined />} aria-label={t('content_studio.activity.video.removeCue', { n: i + 1 })} onClick={() => set({ embedded_questions: cues.filter((_, k) => k !== i) })} /> },
          ]}
        />
        {!readOnly && <Button onClick={() => set({ embedded_questions: [...cues, { cue_id: `q${cues.length + 1}`, at_ms: 0, required: true, allow_skip_after_wrong: true }] })}>{t('content_studio.activity.video.addCue')}</Button>}
        <QuestionPicker
          open={picking !== null}
          onClose={() => setPicking(null)}
          onPick={(q) => {
            if (picking !== null && q.content_version_id) set({ embedded_questions: upd(cues, picking, { question_version_id: q.content_version_id }) });
            setPicking(null);
          }}
        />
      </Card>

      <Card size="small" title={t('content_studio.activity.video.branching')}>
        <Flex vertical gap="small">
          {segments.map((s, i) => (
            <Flex key={i} gap="small" align="center" data-testid={`segment-${i}`}>
              <Input aria-label={t('content_studio.activity.video.segmentIdOf', { n: i + 1 })} maxLength={64} disabled={readOnly} value={s.segment_id} style={{ width: 160 }} onChange={(e) => set({ segments: upd(segments, i, { segment_id: e.target.value }) })} />
              <MediaPicker id={`segment_${i}`} kind="video" value={s.video_asset_id} disabled={readOnly} onChange={(v) => set({ segments: upd(segments, i, { video_asset_id: v }) })} onStatus={onMediaStatus} />
              <Checkbox disabled={readOnly} checked={!!s.is_ending} onChange={(e) => set({ segments: upd<Segment>(segments, i, { is_ending: e.target.checked }) })}>
                {t('content_studio.activity.video.ending')}
              </Checkbox>
            </Flex>
          ))}
          {!readOnly && <Button onClick={() => set({ segments: [...segments, { segment_id: `seg${segments.length + 1}`, is_ending: true }] })}>{t('content_studio.activity.video.addSegment')}</Button>}
          {branches.map((b, i) => (
            <Card key={i} size="small" data-testid={`branch-${i}`} title={t('content_studio.activity.video.branchOf', { n: i + 1 })}>
              <Flex gap="small" wrap>
                <InputNumber aria-label={t('content_studio.activity.video.branchAtOf', { n: i + 1 })} min={0} disabled={readOnly} value={sec(b.at_ms)} onChange={(v) => set({ branches: upd(branches, i, { at_ms: ms(v) }) })} />
                <Input aria-label={t('content_studio.activity.video.branchPromptOf', { n: i + 1 })} maxLength={128} showCount disabled={readOnly} value={b.prompt?.vi ?? ''} onChange={(e) => set({ branches: upd(branches, i, { prompt: { vi: e.target.value } }) })} />
              </Flex>
              {b.options.map((o, j) => (
                <Flex key={j} gap="small">
                  <Input aria-label={t('content_studio.activity.video.optionLabelOf', { n: j + 1 })} maxLength={128} disabled={readOnly} value={o.label?.vi ?? ''} onChange={(e) => set({ branches: upd(branches, i, { options: upd(b.options, j, { label: { vi: e.target.value } }) }) })} />
                  <SearchSelect aria-label={t('content_studio.activity.video.optionTargetOf', { n: j + 1 })} disabled={readOnly} style={{ width: 160 }} value={o.target_segment_id || undefined} options={segments.map((s) => ({ value: s.segment_id, label: s.segment_id }))} onChange={(v: string) => set({ branches: upd(branches, i, { options: upd(b.options, j, { target_segment_id: v }) }) })} />
                </Flex>
              ))}
              {!readOnly && b.options.length < 4 && <Button size="small" onClick={() => set({ branches: upd(branches, i, { options: [...b.options, { option_id: `o${b.options.length + 1}`, label: {}, target_segment_id: '' }] }) })}>{t('content_studio.activity.video.addOption')}</Button>}
              {msg(`branches/${i}/options`) && <Typography.Text type="danger">{msg(`branches/${i}/options`)}</Typography.Text>}
            </Card>
          ))}
          {!readOnly && <Button onClick={() => set({ branches: [...branches, { branch_point_id: `bp${branches.length + 1}`, at_ms: 0, prompt: {}, options: [] } as Branch] })}>{t('content_studio.activity.video.addBranch')}</Button>}
          {segmentMsg && <Typography.Text type="danger">{segmentMsg}</Typography.Text>}
        </Flex>
      </Card>

      <Card size="small" title={t('content_studio.activity.video.rules')}>
        <Form layout="vertical" component="div" disabled={readOnly}>
          <Form.Item label={t('content_studio.activity.video.seek')}>
            <SearchSelect aria-label={t('content_studio.activity.video.seek')} value={interaction.allow_seek as string | undefined} options={SEEKS.map((s) => ({ value: s, label: t(`content_studio.activity.video.seekPolicy.${s}`) }))} onChange={(v: string) => onChange({ ...config, interaction: { ...interaction, allow_seek: v } })} />
          </Form.Item>
          <Form.Item label={t('content_studio.activity.video.rates')}>
            <SearchSelect mode="multiple" aria-label={t('content_studio.activity.video.rates')} value={(interaction.playback_rates as number[] | undefined) ?? []} options={RATES.map((r) => ({ value: r, label: `${r}×` }))} onChange={(v: number[]) => onChange({ ...config, interaction: { ...interaction, playback_rates: v } })} />
          </Form.Item>
          <Flex gap="middle" align="center">
            <Form.Item label={t('content_studio.activity.video.minWatched')}>
              <InputNumber aria-label={t('content_studio.activity.video.minWatched')} min={0} max={1} step={0.05} value={completion.min_watched_ratio as number | undefined} onChange={(v) => onChange({ ...config, completion_rule: { ...completion, mode: 'min_watched_ratio', min_watched_ratio: v ?? 0 } })} />
            </Form.Item>
            <Checkbox checked={(completion.require_required_cues as boolean | undefined) ?? true} onChange={(e) => onChange({ ...config, completion_rule: { ...completion, require_required_cues: e.target.checked } })}>
              {t('content_studio.activity.video.requireCues')}
            </Checkbox>
          </Flex>
        </Form>
      </Card>

      {content.video_asset_id && (
        <Card size="small" title={t('content_studio.activity.video.preview')}>
          <VideoPreview assetId={content.video_asset_id} content={content} />
        </Card>
      )}
    </Flex>
  );
}
