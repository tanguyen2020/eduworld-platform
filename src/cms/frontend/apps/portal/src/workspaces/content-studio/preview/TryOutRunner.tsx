// TryOutRunner (F054-AC-10, ADMIN_PORTAL §5): "Thử làm" one rendered question like a student — capture an answer,
// submitPreviewResponse (scored by the server, no attempt, no learning event, no XP — ADR-0031), then show right /
// wrong, the feedback of the chosen options, the explanation and the hints one level at a time. No answer key ever
// reaches the portal before the server scores (AGENTS R6).
import { ArrowDownOutlined, ArrowUpOutlined } from '@ant-design/icons';
import { Alert, Button, Checkbox, Flex, Input, List, Radio, Space, Tag, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { RichView } from './RichView';
import { buildResponse, canTryOut, emptyAnswer, moveItem, type TryOutAnswer } from './tryOutResponse';
import { useTryOut, type PreviewLang, type PreviewResult, type QuestionRender } from './usePreview';

interface Opt {
  id: string;
  content?: unknown;
  text?: unknown;
}

const list = (rp: Record<string, unknown>, k: string): Opt[] => (Array.isArray(rp[k]) ? (rp[k] as Opt[]) : []);

export interface TryOutRunnerProps {
  sessionId: string;
  item: QuestionRender;
  language: PreviewLang;
  contentLanguage: PreviewLang;
}

export function TryOutRunner({ sessionId, item, language, contentLanguage }: TryOutRunnerProps) {
  const { t } = useTranslation();
  const rp = item.render_payload;
  const [answer, setAnswer] = useState<TryOutAnswer | null>(() => emptyAnswer(item.type, rp));
  const [result, setResult] = useState<PreviewResult | null>(null);
  const [hints, setHints] = useState(0);
  const tryOut = useTryOut(sessionId);
  const rich = (v: unknown) => <RichView value={v} language={language} contentLanguage={contentLanguage} />;
  if (!canTryOut(item.type) || !answer) return <Alert type="info" showIcon message={t('content_studio.preview.tryOutUnsupported', { type: item.type })} />;
  const response = buildResponse(answer, rp);
  const set = (a: TryOutAnswer) => {
    setAnswer(a);
    setResult(null);
  };
  const options = list(rp, 'options');

  const input = (() => {
    switch (answer.kind) {
      case 'single':
        return (
          <Radio.Group value={answer.optionId} onChange={(e) => set({ kind: 'single', optionId: e.target.value as string })} aria-label={t('content_studio.preview.answer')}>
            <Space direction="vertical">
              {options.map((o) => (
                <Radio key={o.id} value={o.id}>
                  {rich(o.content)}
                </Radio>
              ))}
            </Space>
          </Radio.Group>
        );
      case 'multiple':
        return (
          <Checkbox.Group value={answer.optionIds} onChange={(v) => set({ kind: 'multiple', optionIds: v as string[] })} aria-label={t('content_studio.preview.answer')}>
            <Space direction="vertical">
              {options.map((o) => (
                <Checkbox key={o.id} value={o.id}>
                  {rich(o.content)}
                </Checkbox>
              ))}
            </Space>
          </Checkbox.Group>
        );
      case 'true_false': {
        const statements = list(rp, 'statements');
        const tf = (value: boolean | undefined, onChange: (v: boolean) => void, label: string) => (
          <Radio.Group value={value} onChange={(e) => onChange(e.target.value as boolean)} aria-label={label}>
            <Radio value>{t('content_studio.preview.true')}</Radio>
            <Radio value={false}>{t('content_studio.preview.false')}</Radio>
          </Radio.Group>
        );
        if (statements.length === 0) return tf(answer.value, (v) => set({ ...answer, value: v }), t('content_studio.preview.answer'));
        return (
          <List
            size="small"
            dataSource={statements}
            renderItem={(s, i) => (
              <List.Item key={s.id}>
                <Flex vertical gap="small">
                  {rich(s.content)}
                  {tf(answer.statements[s.id], (v) => set({ ...answer, statements: { ...answer.statements, [s.id]: v } }), t('content_studio.preview.statement', { n: i + 1 }))}
                </Flex>
              </List.Item>
            )}
          />
        );
      }
      case 'blanks':
        return (
          <Flex vertical gap="small">
            {Object.keys(answer.blanks).map((b, i) => (
              <Input key={b} aria-label={t('content_studio.preview.blank', { n: i + 1 })} placeholder={t('content_studio.preview.blank', { n: i + 1 })} maxLength={200} value={answer.blanks[b]} onChange={(e) => set({ kind: 'blanks', blanks: { ...answer.blanks, [b]: e.target.value } })} />
            ))}
          </Flex>
        );
      case 'text':
        return <Input aria-label={t('content_studio.preview.answer')} maxLength={500} value={answer.text} onChange={(e) => set({ kind: 'text', text: e.target.value })} />;
      case 'numeric':
        return <Input aria-label={t('content_studio.preview.answer')} maxLength={64} value={answer.value} onChange={(e) => set({ kind: 'numeric', value: e.target.value })} inputMode="decimal" />;
      case 'ordering': {
        const items = new Map(list(rp, 'items').map((x) => [x.id, x]));
        return (
          <List
            size="small"
            bordered
            dataSource={answer.order}
            renderItem={(id, i) => (
              <List.Item
                key={id}
                actions={[
                  <Button key="up" size="small" disabled={i === 0} onClick={() => set({ kind: 'ordering', order: moveItem(answer.order, i, -1) })} aria-label={t('content_studio.preview.moveUp', { n: i + 1 })}>
                    <ArrowUpOutlined />
                  </Button>,
                  <Button key="down" size="small" disabled={i === answer.order.length - 1} onClick={() => set({ kind: 'ordering', order: moveItem(answer.order, i, 1) })} aria-label={t('content_studio.preview.moveDown', { n: i + 1 })}>
                    <ArrowDownOutlined />
                  </Button>,
                ]}
              >
                {rich(items.get(id)?.content)}
              </List.Item>
            )}
          />
        );
      }
    }
  })();

  const hintLevels = [...(result?.hint_levels ?? [])].sort((a, b) => a.level - b.level);
  return (
    <Flex vertical gap="small" data-testid={`try-out-${item.position}`}>
      {input}
      <Button
        type="primary"
        disabled={!response}
        loading={tryOut.isPending}
        onClick={() =>
          response &&
          tryOut.mutate(
            { position: item.position, response },
            {
              onSuccess: (r) => {
                setResult(r);
                setHints(r.is_correct ? 0 : Math.min(1, r.hint_levels?.length ?? 0));
              },
            },
          )
        }
      >
        {t('content_studio.preview.check')}
      </Button>
      {tryOut.error ? <ProblemMessage error={tryOut.error} /> : null}
      {result && (
        <Flex vertical gap="small" data-testid="try-out-result">
          <span>
            <Tag color={result.is_correct ? 'success' : 'error'}>{result.is_correct ? t('content_studio.preview.correct') : t('content_studio.preview.incorrect')}</Tag>
            <Typography.Text>{t('content_studio.preview.score', { score: result.score, max: result.max_score })}</Typography.Text>
          </span>
          {(result.feedback ?? []).map((f) => (
            <Alert key={f.option_id} type="info" message={t('content_studio.preview.optionFeedback')} description={rich(f.feedback)} />
          ))}
          {hintLevels.slice(0, hints).map((h) => (
            <Alert key={h.level} type="warning" message={t('content_studio.preview.hint', { level: h.level })} description={rich(h.hint)} />
          ))}
          {hints < hintLevels.length && <Button onClick={() => setHints((n) => n + 1)}>{t('content_studio.preview.nextHint', { level: hintLevels[hints]!.level })}</Button>}
          {result.explanation ? <Alert type="success" message={t('content_studio.preview.explanation')} description={rich(result.explanation)} /> : null}
        </Flex>
      )}
    </Flex>
  );
}

export type { PreviewResult };
