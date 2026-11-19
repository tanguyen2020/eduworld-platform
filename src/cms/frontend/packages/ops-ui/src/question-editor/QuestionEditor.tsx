// QuestionEditor (F053-AC-01…AC-04, AC-16, AC-20; QUESTION_BANK §2–§3): type picker (11 types; switching with an
// answer asks first and drops what is incompatible), stem (rich content + spoken stem `audio_ref`, required for
// grades 1–3), the type editor, explanation, up to 5 hints and the scoring rule. The value is the authoring payload
// (mappers). Presentational: the page saves, validates with question.schema.json and calls the workflow.
import { Alert, Divider, Flex, Form, Modal, Select, Typography } from 'antd';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import type { ContentLang } from '../i18n-field/translationStatus';
import { OPS_UI_NS } from '../i18n';
import { RichContentEditor, type MediaSlotArgs } from '../rich-content/RichContentEditor';
import { HintsField } from './HintsField';
import { ADVANCED_TYPES, changeType, expressionInputWarning, needsTypeChangeConfirm, QUESTION_TYPES, scoringMethods, type QuestionDraft, type QuestionType } from './mappers';
import { TYPE_EDITORS } from './typeEditors';

export interface QuestionEditorProps {
  value: QuestionDraft;
  onChange: (value: QuestionDraft) => void;
  contentLanguage: ContentLang;
  readOnly?: boolean;
  /** Field errors by dotted path (`stem.blocks.0.text`, `answer_key.correct_option_id`…). */
  errors?: Readonly<Record<string, string>>;
  outdatedFields?: ReadonlySet<string>;
  mediaSlot?: (args: MediaSlotArgs) => ReactNode;
  /** grade_levels ∩ {1,2,3} ≠ ∅ and no `audio_exempt_reason` (CONTENT_MODEL §8.1). */
  stemAudioRequired?: boolean;
  /** The type cannot change any more (existing question: a new type = a new question). */
  typeLocked?: boolean;
  /** Confirm dialog (tests inject one); default Ant Design Modal.confirm. */
  confirm?: (message: string) => Promise<boolean>;
}

const AUDIO = 'audio';
const STEM_AUDIO = 'stem.audio_ref';

export function QuestionEditor({ value, onChange, contentLanguage, readOnly = false, errors = {}, outdatedFields, mediaSlot, stemAudioRequired = false, typeLocked = false, confirm }: QuestionEditorProps) {
  const { t } = useTranslation(OPS_UI_NS);
  const Editor = TYPE_EDITORS[value.type];
  const ask =
    confirm ??
    ((message: string) =>
      new Promise<boolean>((resolve) =>
        Modal.confirm({ title: t('question.changeTypeTitle'), content: message, okText: t('question.changeTypeOk'), cancelText: t('math.cancel'), onOk: () => resolve(true), onCancel: () => resolve(false) }),
      ));
  const onType = async (type: QuestionType) => {
    if (needsTypeChangeConfirm(value, type) && !(await ask(t('question.changeTypeMessage')))) return;
    onChange(changeType(value, type));
  };
  const methods = scoringMethods(value.type);
  const stemAudio = mediaSlot?.({ kind: AUDIO, value: value.stem.audio_ref, onChange: (audio_ref) => onChange({ ...value, stem: { ...value.stem, audio_ref } }), id: STEM_AUDIO });
  return (
    <Form layout="vertical" component="div" disabled={readOnly}>
      <Flex vertical gap="middle">
        <Form.Item label={t('question.type')}>
          <Select<QuestionType>
            aria-label={t('question.type')}
            value={value.type}
            disabled={readOnly || typeLocked}
            onChange={(v) => void onType(v)}
            options={QUESTION_TYPES.map((q) => ({ value: q, label: ADVANCED_TYPES.includes(q) ? t('question.typeAdvanced', { type: t(`question.types.${q}`) }) : t(`question.types.${q}`) }))}
          />
        </Form.Item>
        {expressionInputWarning(value) && <Alert type="warning" showIcon message={t('question.expressionWarning')} data-testid="expression-warning" />}
        <RichContentEditor id="stem" label={t('question.stem')} value={value.stem} onChange={(stem) => onChange({ ...value, stem })} contentLanguage={contentLanguage} readOnly={readOnly} errors={errors} outdatedFields={outdatedFields} mediaSlot={mediaSlot} required />
        <Form.Item
          label={t('question.stemAudio')}
          required={stemAudioRequired}
          validateStatus={errors['stem.audio_ref'] || (stemAudioRequired && !value.stem.audio_ref) ? 'error' : undefined}
          help={errors['stem.audio_ref'] ?? (stemAudioRequired && !value.stem.audio_ref ? t('question.stemAudioRequired') : undefined)}
          data-field="stem.audio_ref"
        >
          {stemAudio ?? (
            <Typography.Text type="secondary">{value.stem.audio_ref ?? t('richContent.noMedia')}</Typography.Text>
          )}
        </Form.Item>
        <Divider orientation="left">{t('question.answerSection')}</Divider>
        <Editor
          draft={value}
          set={(patch) => onChange({ ...value, ...patch })}
          lang={contentLanguage}
          readOnly={readOnly}
          errors={errors}
          mediaSlot={mediaSlot}
        />
        {methods.length > 0 && (
          <Form.Item label={t('question.scoring')}>
            <Select
              aria-label={t('question.scoring')}
              value={value.scoring_rule.mode === 'partial' ? value.scoring_rule.partial_method : 'all_or_nothing'}
              options={[{ value: 'all_or_nothing', label: t('question.scoringMethods.all_or_nothing') }, ...methods.map((m) => ({ value: m, label: t(`question.scoringMethods.${m}`) }))]}
              onChange={(m: string) => onChange({ ...value, scoring_rule: m === 'all_or_nothing' ? { mode: 'all_or_nothing' } : { ...value.scoring_rule, mode: 'partial', partial_method: m as never } })}
            />
          </Form.Item>
        )}
        <Divider orientation="left">{t('question.explanationSection')}</Divider>
        <RichContentEditor id="explanation" label={t('question.explanation')} value={value.explanation} onChange={(explanation) => onChange({ ...value, explanation })} contentLanguage={contentLanguage} readOnly={readOnly} errors={errors} outdatedFields={outdatedFields} mediaSlot={mediaSlot} />
        <HintsField value={value.hints ?? []} onChange={(hints) => onChange({ ...value, hints })} contentLanguage={contentLanguage} readOnly={readOnly} errors={errors} mediaSlot={mediaSlot} />
      </Flex>
    </Form>
  );
}
