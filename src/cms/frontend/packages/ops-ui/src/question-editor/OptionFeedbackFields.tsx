// OptionFeedbackFields (F053-AC-03, FR-QB-013, QUESTION_BANK §2): for a WRONG option of single / multiple choice —
// `misconception_tag` (≤ 64) and a bilingual `feedback` (≤ 500). Both are part of the answer: the server strips them
// from the student DTO and returns the feedback only after scoring (FeedbackTiming).
import { Flex, Form, Input } from 'antd';
import { useTranslation } from 'react-i18next';
import { BilingualField } from '../i18n-field/BilingualField';
import type { ContentLang } from '../i18n-field/translationStatus';
import { OPS_UI_NS } from '../i18n';
import { LIMITS, type OptionItem } from './mappers';

export interface OptionFeedbackFieldsProps {
  id: string;
  option: OptionItem;
  onChange: (option: OptionItem) => void;
  contentLanguage: ContentLang;
  readOnly?: boolean;
  errors?: Readonly<Record<string, string>>;
}

export function OptionFeedbackFields({ id, option, onChange, contentLanguage, readOnly = false, errors = {} }: OptionFeedbackFieldsProps) {
  const { t } = useTranslation(OPS_UI_NS);
  return (
    <Flex vertical gap="small" data-testid={`feedback-${option.id}`}>
      <Form.Item label={t('question.misconceptionTag')} validateStatus={errors[`${id}.misconception_tag`] ? 'error' : undefined} help={errors[`${id}.misconception_tag`]}>
        <Input
          id={`${id}.misconception_tag`}
          value={option.misconception_tag ?? ''}
          maxLength={LIMITS.misconceptionTag}
          showCount
          readOnly={readOnly}
          onChange={(e) => onChange({ ...option, misconception_tag: e.target.value || undefined })}
        />
      </Form.Item>
      <BilingualField
        id={`${id}.feedback`}
        label={t('question.optionFeedback')}
        value={option.feedback}
        onChange={(v) => onChange({ ...option, feedback: v })}
        contentLanguage={contentLanguage}
        maxLength={LIMITS.feedback}
        multiline
        readOnly={readOnly}
        error={errors[`${id}.feedback`]}
      />
    </Flex>
  );
}
