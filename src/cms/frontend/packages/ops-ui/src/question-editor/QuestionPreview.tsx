// QuestionPreview (F053-AC-03, AC-19): "như học sinh thấy" — stem and the items of a STUDENT render payload, formulas
// with MathJax 4. It never shows feedback, misconception tags, hints, explanation or answers: the input is passed
// through toStudentView() and only display keys are read (the portal feeds it the server preview render).
import { Card, Flex, List, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { textIn, type ContentLang, type I18nTextValue } from '../i18n-field/translationStatus';
import { OPS_UI_NS } from '../i18n';
import { MathText } from '../math/MathText';
import type { RichContent } from '../rich-content/types';
import { toStudentView, type QuestionType } from './mappers';

export interface QuestionPreviewProps {
  type: QuestionType;
  stem?: RichContent;
  renderPayload: Record<string, unknown>;
  /** Display language (vi / en switch of the preview). */
  language: ContentLang;
  contentLanguage: ContentLang;
}

function Rich({ content, language, contentLanguage }: { content?: RichContent; language: ContentLang; contentLanguage: ContentLang }) {
  const { t } = useTranslation(OPS_UI_NS);
  if (!content) return null;
  return (
    <Flex vertical gap="small">
      {content.blocks.map((b, i) => {
        if (b.type === 'math') return <MathText key={i} latex={b.latex ?? ''} display language={language} />;
        if (b.type === 'image') return <Typography.Text key={i} type="secondary">{t('preview.image', { alt: b.alt ?? '' })}</Typography.Text>;
        if (b.type === 'audio') return <Typography.Text key={i} type="secondary">{t('preview.audio')}</Typography.Text>;
        if (b.type === 'list') return <ul key={i}>{(b.items ?? []).map((it, k) => <li key={k}><MathText text={textIn(it, language, contentLanguage)} language={language} /></li>)}</ul>;
        return (
          <Typography.Paragraph key={i}>
            <MathText text={textIn(b.text as I18nTextValue | undefined, language, contentLanguage)} language={language} />
          </Typography.Paragraph>
        );
      })}
    </Flex>
  );
}

const LISTS = ['options', 'statements', 'left', 'right', 'items', 'draggables', 'labels'] as const;

export function QuestionPreview({ type, stem, renderPayload, language, contentLanguage }: QuestionPreviewProps) {
  const { t } = useTranslation(OPS_UI_NS);
  const view = toStudentView({ type, stem: stem ?? { blocks: [] }, render_payload: renderPayload });
  const rp = view.render_payload;
  return (
    <Card title={t('preview.title')} data-testid="student-preview">
      <Flex vertical gap="middle">
        <Rich content={view.stem} language={language} contentLanguage={contentLanguage} />
        {LISTS.filter((k) => Array.isArray(rp[k])).map((k) => (
          <List
            key={k}
            size="small"
            bordered
            dataSource={rp[k] as { id: string; content?: RichContent }[]}
            renderItem={(o) => (
              <List.Item>
                <Rich content={o.content} language={language} contentLanguage={contentLanguage} />
              </List.Item>
            )}
          />
        ))}
        {Array.isArray(rp.zones) && (
          <List size="small" bordered dataSource={rp.zones as { id: string; label: I18nTextValue }[]} renderItem={(z) => <List.Item>{textIn(z.label, language, contentLanguage)}</List.Item>} />
        )}
        {Array.isArray(rp.blanks) && <Typography.Text type="secondary">{t('preview.blanks', { n: (rp.blanks as unknown[]).length })}</Typography.Text>}
        {type === 'true_false' && !Array.isArray(rp.statements) && <Typography.Text>{t('preview.trueFalse')}</Typography.Text>}
        {type === 'numeric' && <Typography.Text type="secondary">{t('preview.numeric', { unit: (rp.unit_label as string) ?? '' })}</Typography.Text>}
      </Flex>
    </Card>
  );
}
