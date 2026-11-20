// EfficacyInstrumentField (F035 US8, F035-AC-36, AC-37; LEARNING_EFFICACY_PLAN §3.2): the "Đo hiệu quả" box of the
// question and blueprint editors. Shown only in the platform scope (no session tenant) and only to whoever holds the
// edit action of the editor; never in a tenant scope (R20). Display only: the backend accepts the flag only for
// platform content, `exam_only` and non-parameterised questions, fixed blueprints (422 VALIDATION_FAILED otherwise).
import { Checkbox, Form, Tag } from 'antd';
import { useTranslation } from 'react-i18next';
import type { ActionKey } from '../../../generated/menu-catalog';
import { ActionGate } from '../../../permissions/ActionGate';
import { useTenantSource } from '../../../shared/auth/TenantContext';

export interface EfficacyInstrumentFieldProps {
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
  /** Edit action of the editor: question bank or assessments. */
  actionKey: Extract<ActionKey, 'action.content_studio.question_bank.edit' | 'action.content_studio.assessments.edit'>;
  /** Help text: question (exam_only, not parameterised) or blueprint (fixed variants). */
  kind: 'question' | 'blueprint';
}

export function EfficacyInstrumentField({ checked, onChange, disabled, actionKey, kind }: EfficacyInstrumentFieldProps) {
  const { t } = useTranslation();
  const tenant = useTenantSource();
  if (tenant.getTenantId()) return null;
  return (
    <ActionGate actionKey={actionKey}>
      <Form.Item label={t('content_studio.efficacy.title')} extra={t(`content_studio.efficacy.help.${kind}`)} data-field="efficacy_instrument">
        <Checkbox checked={checked} disabled={disabled} onChange={(e) => onChange(e.target.checked)}>
          {t('content_studio.efficacy.label')}
        </Checkbox>
        {checked ? <Tag>{t('content_studio.efficacy.label')}</Tag> : null}
      </Form.Item>
    </ActionGate>
  );
}
