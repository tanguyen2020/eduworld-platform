// PreviewPanel (F054-AC-10, US-CMS-002, ADMIN_PORTAL §5): "Xem trước" of a chosen version — vi / en / side by side
// (one server preview session per display language, created on demand) rendered with the shared web renderer
// (ops-ui QuestionPreview, MathJax 4), and "Thử làm" (TryOutRunner) scored by the server without an attempt.
import { QuestionPreview, type RichContent } from '@eduworld/ops-ui/content';
import { Alert, Button, Card, Col, Flex, Row, Segmented, Skeleton, Switch, theme, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import type { ItemRef } from '../release/useRelease';
import { TryOutRunner } from './TryOutRunner';
import { usePreviewSession, type PreviewLang } from './usePreview';

export interface PreviewPanelProps {
  item: ItemRef;
  version: number;
  contentLanguage?: PreviewLang;
}

function PreviewColumn({ item, version, lang, contentLanguage, tryOut }: { item: ItemRef; version: number; lang: PreviewLang; contentLanguage: PreviewLang; tryOut: boolean }) {
  const { t } = useTranslation();
  const s = usePreviewSession(item, version, lang, true);
  if (s.isPending) return <Skeleton active />;
  if (s.error) return <ProblemMessage error={s.error} />;
  const { items = [], render } = s.data;
  return (
    <Flex vertical gap="middle" data-testid={`preview-${lang}`}>
      {render?.translation_missing && <Alert type="info" showIcon message={t('content_studio.preview.translationMissing')} />}
      {render && (
        <Card size="small" title={render.title}>
          {render.instructions ? <Typography.Paragraph>{render.instructions}</Typography.Paragraph> : null}
        </Card>
      )}
      {items.length === 0 && !render && <Typography.Text type="secondary">{t('content_studio.preview.empty')}</Typography.Text>}
      {items.map((it) => (
        <Card key={it.position} size="small" title={t('content_studio.preview.itemTitle', { n: it.position })}>
          {tryOut ? (
            <Flex vertical gap="small">
              <QuestionPreview type={it.type} stem={(it.render_payload as { stem?: RichContent }).stem} renderPayload={{ ...it.render_payload, options: undefined, statements: undefined, items: undefined }} language={lang} contentLanguage={contentLanguage} />
              <TryOutRunner sessionId={s.data.preview_session_id} item={it} language={lang} contentLanguage={contentLanguage} />
            </Flex>
          ) : (
            <QuestionPreview type={it.type} stem={(it.render_payload as { stem?: RichContent }).stem} renderPayload={it.render_payload} language={lang} contentLanguage={contentLanguage} />
          )}
        </Card>
      ))}
    </Flex>
  );
}

export function PreviewPanel({ item, version, contentLanguage = 'vi' }: PreviewPanelProps) {
  const { t } = useTranslation();
  const [mode, setMode] = useState<PreviewLang | 'both'>(contentLanguage);
  const [started, setStarted] = useState(false);
  const [tryOut, setTryOut] = useState(false);
  const { token } = theme.useToken();
  const langs: PreviewLang[] = mode === 'both' ? ['vi', 'en'] : [mode];
  return (
    <section aria-label={t('content_studio.preview.title')} data-testid="preview-panel">
      <Flex gap="middle" align="center" wrap>
        <Segmented
          aria-label={t('content_studio.preview.language')}
          value={mode}
          onChange={(v) => setMode(v as PreviewLang | 'both')}
          options={[
            { value: 'vi', label: t('content_studio.preview.vi') },
            { value: 'en', label: t('content_studio.preview.en') },
            { value: 'both', label: t('content_studio.preview.both') },
          ]}
        />
        <Flex gap="small" align="center">
          <Switch checked={tryOut} onChange={setTryOut} aria-label={t('content_studio.preview.tryOut')} />
          <Typography.Text>{t('content_studio.preview.tryOut')}</Typography.Text>
        </Flex>
        {!started && (
          <ActionGate actionKey="action.content_studio.reviews.preview">
            <Button type="primary" onClick={() => setStarted(true)}>
              {t('content_studio.preview.open', { version })}
            </Button>
          </ActionGate>
        )}
      </Flex>
      <Typography.Paragraph type="secondary">{t('content_studio.preview.noAttempt')}</Typography.Paragraph>
      {started && (
        <Row gutter={[token.margin, token.margin]}>
          {langs.map((l) => (
            <Col key={l} xs={24} lg={langs.length > 1 ? 12 : 24}>
              <PreviewColumn item={item} version={version} lang={l} contentLanguage={contentLanguage} tryOut={tryOut} />
            </Col>
          ))}
        </Row>
      )}
    </section>
  );
}
