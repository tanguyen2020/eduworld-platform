// EventTemplateEditor (F079-AC-15, FR-EVT-006, event-template.schema.json): an event template offered to schools —
// `allowed_params` (which parameters a school may fill; others → 422 EVENT_TEMPLATE_PARAM_NOT_ALLOWED) shown first with
// its explanation, `max_duration_days` capped by the server (`school_event_max_duration_days`), then the rest of the
// template from its schema.
import { Alert, Flex } from 'antd';
import { useTranslation } from 'react-i18next';
import { GenericSchemaBuilder } from '../GenericSchemaBuilder';
import type { BuilderProps } from '../types';

export function EventTemplateEditor(props: BuilderProps) {
  const { t } = useTranslation();
  return (
    <Flex vertical gap="middle" data-testid="event-template-editor">
      <Alert type="info" showIcon message={t('gameOps.template.allowedParamsHint')} />
      <GenericSchemaBuilder {...props} />
    </Flex>
  );
}
