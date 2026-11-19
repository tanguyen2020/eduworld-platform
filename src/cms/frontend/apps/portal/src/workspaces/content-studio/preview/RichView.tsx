// RichView: read-only display of a server rich content (`{blocks: […]}`, CONTENT_MODEL §9) or a bilingual text
// (`{vi, en}` / string) in the chosen language, formulas with MathJax 4 (ops-ui MathText). Used by the try-out
// feedback, explanation and hints (F054-AC-10). Presentational only.
import { MathText, textIn, type ContentLang, type I18nTextValue, type RichContent } from '@eduworld/ops-ui/content';
import { Flex, Typography } from 'antd';
import { useTranslation } from 'react-i18next';

export function RichView({ value, language, contentLanguage }: { value: unknown; language: ContentLang; contentLanguage: ContentLang }) {
  const { t } = useTranslation();
  if (value === undefined || value === null) return null;
  const rich = value as Partial<RichContent>;
  if (!Array.isArray(rich.blocks)) return <MathText text={textIn(value as I18nTextValue, language, contentLanguage)} language={language} />;
  return (
    <Flex vertical gap="small">
      {rich.blocks.map((b, i) => {
        if (b.type === 'math') return <MathText key={i} latex={b.latex ?? ''} display language={language} />;
        if (b.type === 'image' || b.type === 'audio' || b.type === 'video') return <Typography.Text key={i} type="secondary">{t('content_studio.preview.mediaBlock', { kind: b.type })}</Typography.Text>;
        if (b.type === 'list')
          return (
            <ul key={i}>
              {(b.items ?? []).map((it, k) => (
                <li key={k}>
                  <MathText text={textIn(it, language, contentLanguage)} language={language} />
                </li>
              ))}
            </ul>
          );
        return (
          <Typography.Paragraph key={i}>
            <MathText text={textIn(b.text as I18nTextValue | undefined, language, contentLanguage)} language={language} />
          </Typography.Paragraph>
        );
      })}
    </Flex>
  );
}
