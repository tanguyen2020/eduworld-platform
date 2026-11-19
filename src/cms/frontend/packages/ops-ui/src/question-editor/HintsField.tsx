// HintsField (F053-AC-03, FR-QB-013): up to 5 hint levels (rich content), shown to students one level at a time by the
// server (`hints:next`), never in the render payload.
import { Button, Card, Flex, Typography } from 'antd';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import type { ContentLang } from '../i18n-field/translationStatus';
import { OPS_UI_NS } from '../i18n';
import { RichContentEditor, type MediaSlotArgs } from '../rich-content/RichContentEditor';
import { emptyRichContent, type RichContent } from '../rich-content/types';
import { LIMITS } from './mappers';

export interface HintsFieldProps {
  value: RichContent[];
  onChange: (hints: RichContent[]) => void;
  contentLanguage: ContentLang;
  readOnly?: boolean;
  errors?: Readonly<Record<string, string>>;
  mediaSlot?: (args: MediaSlotArgs) => ReactNode;
}

export function HintsField({ value, onChange, contentLanguage, readOnly = false, errors, mediaSlot }: HintsFieldProps) {
  const { t } = useTranslation(OPS_UI_NS);
  return (
    <Flex vertical gap="small" data-field="hints">
      <Typography.Text strong>{t('question.hints', { max: LIMITS.hints })}</Typography.Text>
      {value.map((h, i) => (
        <Card
          key={i}
          size="small"
          title={t('question.hintLevel', { n: i + 1 })}
          extra={
            !readOnly && (
              <Button size="small" danger onClick={() => onChange(value.filter((_, j) => j !== i))}>
                {t('question.removeHint')}
              </Button>
            )
          }
        >
          <RichContentEditor
            id={`hints.${i}`}
            label={t('question.hintLevel', { n: i + 1 })}
            value={h}
            onChange={(v) => onChange(value.map((x, j) => (j === i ? v : x)))}
            contentLanguage={contentLanguage}
            allowedBlocks={['paragraph', 'math', 'image']}
            readOnly={readOnly}
            errors={errors}
            mediaSlot={mediaSlot}
          />
        </Card>
      ))}
      {!readOnly && (
        <Button disabled={value.length >= LIMITS.hints} onClick={() => onChange([...value, emptyRichContent()])}>
          {t('question.addHint')}
        </Button>
      )}
    </Flex>
  );
}
