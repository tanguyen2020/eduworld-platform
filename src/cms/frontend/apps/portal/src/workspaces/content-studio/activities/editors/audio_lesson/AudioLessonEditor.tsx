// AudioLessonEditor (F066 T045, F066-AC-25, AUDIO_LANGUAGE_LEARNING §2, ADMIN_PORTAL §3.13 Audio Manager): the config
// of `audio_lesson` in the Activity Builder (F058 editor registry) — audio from the media library (upload needs
// media.upload), transcript (required before Gửi duyệt), segments, auto-graded questions from the Question Bank at a
// time (or after the audio), max plays, speed change, transcript display, completion by the heard part. Early checks
// only (audioRules); module learning validates (ACTIVITY_CONFIG_INVALID). No recording, no speaking (ADR-0072).
import { DeleteOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Checkbox, Flex, Form, Input, InputNumber, List, Select, Switch, Table, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { MediaPicker } from '../../../media/MediaPicker';
import { useMediaAsset } from '../../../media/useMediaUpload';
import { QuestionPicker } from '../../../question-sets/SelectionRuleEditor';
import type { Config } from '../../activityModel';
import type { ActivityEditorProps, BlockerContext } from '../../editorRegistry';
import { AUTO_TYPES } from '../../validators/questionSourceRules';
import { audioIssues, MAX_QUESTIONS, MAX_SEGMENTS, type AudioContent, type AudioQuestion, type AudioSegment } from './audioRules';

/** video_completion (CONFIG_DEFAULTS), the default completion of the shared media timeline (F066 Clarifications). */
const COMPLETION = { mode: 'min_watched_ratio', min_watched_ratio: 0.9 } as const;
const TRANSCRIPT_MODES = ['never', 'after_complete', 'always'] as const;

export function audioConfig(): Config {
  return {
    content: { segments: [], questions: [] },
    interaction: { allow_speed_change: true, show_transcript: 'after_complete' },
    evaluation: { strategies: ['completion', 'auto'], max_score: 10 },
    feedback: { timing: 'immediate' },
    completion_rule: { ...COMPLETION },
  };
}

const obj = (v: unknown): Record<string, unknown> => (v && typeof v === 'object' ? (v as Record<string, unknown>) : {});
const sec = (ms: number | undefined) => (ms === undefined ? undefined : Math.round(ms / 100) / 10);
const ms = (s: number | null) => Math.round((s ?? 0) * 1000);

export function audioBlockers(config: Config, ctx: BlockerContext) {
  const c = obj(config.content) as AudioContent;
  // The page does not know the probed duration: relative rules here, the editor shows the duration rules.
  const out = audioIssues(c, undefined).map((i) => ({ field: i.pointer, message: ctx.t(`content_studio.activity.${i.key}`, i.values) }));
  for (const a of [c.audio_asset_id, c.transcript_asset_id].filter(Boolean) as string[]) {
    const st = ctx.mediaStatus[a];
    if (st && st !== 'ready') out.push({ field: '/config/content/audio_asset_id', message: ctx.t('error.MEDIA_NOT_READY') });
  }
  return out;
}

export function AudioLessonEditor({ config, onChange, readOnly, errors, onMediaStatus }: ActivityEditorProps) {
  const { t } = useTranslation();
  const content = obj(config.content) as AudioContent;
  const interaction = obj(config.interaction);
  const completion = obj(config.completion_rule);
  const audio = useMediaAsset(content.audio_asset_id);
  const duration = audio.data?.duration_ms;
  const [picking, setPicking] = useState(false);
  const [refused, setRefused] = useState(false);
  const set = (patch: Partial<AudioContent>) => onChange({ ...config, content: { ...content, ...patch } });
  const issues = audioIssues(content, duration);
  const msg = (p: string) => errors[`/config/content/${p}`] ?? ((i) => (i ? t(`content_studio.activity.${i.key}`, i.values) : undefined))(issues.find((x) => x.pointer === `/config/content/${p}`));
  const segs = content.segments ?? [];
  const qs = content.questions ?? [];
  const upd = <T,>(list: T[], i: number, patch: Partial<T>) => list.map((x, k) => (k === i ? { ...x, ...patch } : x));
  const transcriptMsg = msg('transcript_asset_id');
  const maxPlays = interaction.max_plays as number | undefined;
  return (
    <Flex vertical gap="middle" data-testid="audio-lesson-editor">
      <Card size="small" title={t('content_studio.activity.audio.title')}>
        <Flex vertical gap="small">
          <MediaPicker id="audio_asset_id" kind="audio" value={content.audio_asset_id} disabled={readOnly} onChange={(v) => set({ audio_asset_id: v })} onStatus={onMediaStatus} />
          <Typography.Text data-testid="audio-duration">{t('content_studio.activity.audio.duration', { s: duration ? sec(duration) : '—' })}</Typography.Text>
          <Typography.Text strong>{t('content_studio.activity.audio.transcript')}</Typography.Text>
          <MediaPicker id="transcript_asset_id" kind="subtitle" value={content.transcript_asset_id} disabled={readOnly} onChange={(v) => set({ transcript_asset_id: v })} onStatus={onMediaStatus} />
          {transcriptMsg && <Typography.Text type="warning" data-testid="transcript-issue">{transcriptMsg}</Typography.Text>}
        </Flex>
      </Card>
      <Card size="small" title={t('content_studio.activity.audio.segments')}>
        <Table<AudioSegment>
          size="small"
          pagination={false}
          rowKey={(_, i) => String(i)}
          dataSource={segs}
          data-testid="audio-segments"
          columns={[
            { title: t('content_studio.activity.audio.segmentId'), render: (_, s, i) => <Input aria-label={t('content_studio.activity.audio.segmentIdOf', { n: i + 1 })} maxLength={64} disabled={readOnly} value={s.segment_id} onChange={(e) => set({ segments: upd(segs, i, { segment_id: e.target.value }) })} /> },
            {
              title: t('content_studio.activity.audio.segmentTitle'),
              render: (_, s, i) => {
                const m = msg(`segments/${i}/title`);
                return (
                  <Flex vertical>
                    <Input aria-label={t('content_studio.activity.audio.segmentTitleOf', { n: i + 1 })} maxLength={128} showCount disabled={readOnly} status={m ? 'error' : undefined} value={s.title?.vi ?? ''} onChange={(e) => set({ segments: upd(segs, i, { title: { ...s.title, vi: e.target.value } }) })} />
                    {m && <Typography.Text type="danger">{m}</Typography.Text>}
                  </Flex>
                );
              },
            },
            { title: t('content_studio.activity.video.start'), render: (_, s, i) => <InputNumber aria-label={t('content_studio.activity.audio.startOf', { n: i + 1 })} min={0} step={0.5} disabled={readOnly} value={sec(s.start_ms)} onChange={(v) => set({ segments: upd(segs, i, { start_ms: ms(v) }) })} /> },
            {
              title: t('content_studio.activity.video.end'),
              render: (_, s, i) => {
                const m = msg(`segments/${i}/end_ms`) ?? msg(`segments/${i}/start_ms`) ?? msg(`segments/${i}/segment_id`);
                return (
                  <Flex vertical>
                    <InputNumber aria-label={t('content_studio.activity.audio.endOf', { n: i + 1 })} min={0} step={0.5} disabled={readOnly} status={m ? 'error' : undefined} value={sec(s.end_ms)} onChange={(v) => set({ segments: upd(segs, i, { end_ms: ms(v) }) })} />
                    {m && <Typography.Text type="danger">{m}</Typography.Text>}
                  </Flex>
                );
              },
            },
            { title: '', render: (_, _s, i) => !readOnly && <Button size="small" icon={<DeleteOutlined />} aria-label={t('content_studio.activity.audio.removeSegment', { n: i + 1 })} onClick={() => set({ segments: segs.filter((_, k) => k !== i) })} /> },
          ]}
        />
        {!readOnly && segs.length < MAX_SEGMENTS && <Button onClick={() => set({ segments: [...segs, { segment_id: `s${segs.length + 1}`, title: {}, start_ms: segs.at(-1)?.end_ms ?? 0, end_ms: (segs.at(-1)?.end_ms ?? 0) + 30_000 }] })}>{t('content_studio.activity.audio.addSegment')}</Button>}
      </Card>
      <Card size="small" title={t('content_studio.activity.audio.questions')}>
        <List
          size="small"
          bordered
          data-testid="audio-questions"
          locale={{ emptyText: t('content_studio.activity.source.empty') }}
          dataSource={qs}
          renderItem={(q, i) => (
            <List.Item
              actions={readOnly ? [] : [<Button key="rm" size="small" icon={<DeleteOutlined />} aria-label={t('content_studio.activity.source.remove', { n: i + 1 })} onClick={() => set({ questions: qs.filter((_, k) => k !== i) })} />]}
            >
              <Flex gap="small" align="center">
                <Typography.Text>{t('content_studio.activity.audio.questionItem', { n: i + 1, id: q.question_version_id.slice(-8) })}</Typography.Text>
                <Checkbox disabled={readOnly} checked={q.at_ms !== undefined} onChange={(e) => set({ questions: upd<AudioQuestion>(qs, i, { at_ms: e.target.checked ? 0 : undefined }) })}>
                  {t('content_studio.activity.audio.atTime')}
                </Checkbox>
                {q.at_ms !== undefined && (
                  <InputNumber aria-label={t('content_studio.activity.audio.atOf', { n: i + 1 })} min={0} step={0.5} disabled={readOnly} status={msg(`questions/${i}/at_ms`) ? 'error' : undefined} value={sec(q.at_ms)} onChange={(v) => set({ questions: upd(qs, i, { at_ms: ms(v) }) })} />
                )}
                {!!msg(`questions/${i}/at_ms`) && <Typography.Text type="danger">{msg(`questions/${i}/at_ms`)}</Typography.Text>}
              </Flex>
            </List.Item>
          )}
        />
        {refused && <Alert type="warning" showIcon closable message={t('content_studio.activity.audio.autoOnly')} onClose={() => setRefused(false)} />}
        {!readOnly && qs.length < MAX_QUESTIONS && <Button onClick={() => setPicking(true)}>{t('content_studio.activity.source.add')}</Button>}
        <QuestionPicker
          open={picking}
          onClose={() => setPicking(false)}
          onPick={(q) => {
            if (!q.content_version_id || !AUTO_TYPES.includes(q.type ?? '')) {
              setRefused(true);
              return;
            }
            setRefused(false);
            set({ questions: [...qs, { question_version_id: q.content_version_id }] });
            setPicking(false);
          }}
        />
      </Card>
      <Card size="small" title={t('content_studio.activity.audio.rules')}>
        <Form layout="vertical" component="div" disabled={readOnly}>
          <Flex gap="middle" align="center" wrap>
            <Form.Item label={t('content_studio.activity.audio.maxPlays')} extra={t('content_studio.activity.audio.maxPlaysHelp')}>
              <InputNumber aria-label={t('content_studio.activity.audio.maxPlays')} min={1} max={20} value={maxPlays} onChange={(v) => onChange({ ...config, interaction: { ...interaction, max_plays: v ?? undefined } })} />
            </Form.Item>
            <Flex gap="small" align="center">
              <Switch aria-label={t('content_studio.activity.audio.speed')} checked={(interaction.allow_speed_change as boolean | undefined) ?? true} onChange={(v) => onChange({ ...config, interaction: { ...interaction, allow_speed_change: v } })} />
              <Typography.Text>{t('content_studio.activity.audio.speed')}</Typography.Text>
            </Flex>
            <Form.Item label={t('content_studio.activity.audio.showTranscript')}>
              <Select aria-label={t('content_studio.activity.audio.showTranscript')} style={{ minWidth: 200 }} value={(interaction.show_transcript as string | undefined) ?? 'after_complete'} options={TRANSCRIPT_MODES.map((m) => ({ value: m, label: t(`content_studio.activity.audio.transcriptMode.${m}`) }))} onChange={(v: string) => onChange({ ...config, interaction: { ...interaction, show_transcript: v } })} />
            </Form.Item>
            <Form.Item label={t('content_studio.activity.video.minWatched')}>
              <InputNumber aria-label={t('content_studio.activity.audio.minHeard')} min={0} max={1} step={0.05} value={completion.min_watched_ratio as number | undefined} onChange={(v) => onChange({ ...config, completion_rule: { ...completion, mode: 'min_watched_ratio', min_watched_ratio: v ?? 0 } })} />
            </Form.Item>
          </Flex>
        </Form>
      </Card>
    </Flex>
  );
}
