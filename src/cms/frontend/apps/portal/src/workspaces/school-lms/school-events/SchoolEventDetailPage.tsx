// Detail of a school event (F081-AC-07, AC-08, AC-09, AC-10/11, edge cases; T015): name, template, ICT schedule,
// audience, goal target, status; "Sửa" only while scheduled, "Hủy" while scheduled or live — never while suspended by
// EduWorld ("Tạm dừng bởi EduWorld"); both behind ActionGate. 404 / 403 TENANT_MISMATCH (another school or tenant) →
// "Không tìm thấy" without anything about the event. The progress panel follows (aggregates only).
import { Alert, Button, Descriptions, Flex } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { ActionGate } from '../../../permissions/ActionGate';
import { formatIct } from '../../../shared/release/ict';
import { useCrossTenantScope } from '../../../shared/tenancy/useCrossTenantScope';
import { NotFoundResult, ProblemMessage } from '../common/errors';
import { localizedName } from '../common/format';
import { PageTitle } from '../common/ui';
import { useEventTemplates, useEventTenant, useSchoolEvent } from './api';
import { CancelSchoolEventDialog } from './CancelSchoolEventDialog';
import { SchoolEventProgressPanel } from './SchoolEventProgressPanel';
import { AudienceText, eventPath } from './SchoolEventListPage';
import { EventStatusTag, isCancelable, isEditable, isHidden } from './ui';

export function SchoolEventDetailPage() {
  const { t, i18n } = useTranslation();
  const { eventId } = useParams();
  const navigate = useNavigate();
  const cross = useCrossTenantScope();
  const tenant = useEventTenant();
  const q = useSchoolEvent(eventId);
  const templates = useEventTemplates(tenant.headers, tenant.key, tenant.ready);
  const [cancelling, setCancelling] = useState(false);
  const back = (
    <Link to="/school-lms/school-events">
      <Button>{t('portal.school_lms.common.back')}</Button>
    </Link>
  );
  if (q.error) return isHidden(q.error) ? <NotFoundResult /> : <ProblemMessage error={q.error} />;
  if (!q.data) return null;
  const e = q.data;
  const name = localizedName(e.name, i18n.language);
  const template = templates.items.find((x) => x.template_id === e.template_id);
  return (
    <section>
      <PageTitle
        title={name}
        extra={
          <Flex gap="small" wrap>
            {isEditable(e.status) ? (
              <ActionGate actionKey="action.school_lms.school_events.edit">
                <Link to={eventPath(e, cross.crossTenant, '/edit')}>
                  <Button>{t('portal.school_lms.schoolEvents.detail.edit')}</Button>
                </Link>
              </ActionGate>
            ) : null}
            {isCancelable(e.status) ? (
              <ActionGate actionKey="action.school_lms.school_events.cancel">
                <Button danger onClick={() => setCancelling(true)}>
                  {t('portal.school_lms.schoolEvents.detail.cancel')}
                </Button>
              </ActionGate>
            ) : null}
            {back}
          </Flex>
        }
      />
      <Flex vertical gap="middle">
        {e.status === 'suspended' ? <Alert type="warning" showIcon message={t('portal.school_lms.schoolEvents.detail.suspended')} /> : null}
        <Descriptions bordered column={1} size="small">
          <Descriptions.Item label={t('portal.school_lms.schoolEvents.detail.status')}>
            <EventStatusTag status={e.status} />
          </Descriptions.Item>
          <Descriptions.Item label={t('portal.school_lms.schoolEvents.detail.template')}>{template ? localizedName(template.name, i18n.language) : t('portal.school_lms.schoolEvents.detail.templateUnassigned')}</Descriptions.Item>
          <Descriptions.Item label={t('portal.school_lms.schoolEvents.detail.schedule')}>{`${formatIct(e.starts_at)} – ${formatIct(e.ends_at)} (${t('portal.school_lms.schoolEvents.detail.ict')})`}</Descriptions.Item>
          <Descriptions.Item label={t('portal.school_lms.schoolEvents.detail.audience')}>
            <AudienceText audience={e.audience} />
          </Descriptions.Item>
          {e.community_goal_target ? <Descriptions.Item label={t('portal.school_lms.schoolEvents.detail.goalTarget')}>{e.community_goal_target}</Descriptions.Item> : null}
          {e.description ? <Descriptions.Item label={t('portal.school_lms.schoolEvents.detail.description')}>{localizedName(e.description, i18n.language)}</Descriptions.Item> : null}
        </Descriptions>
        <SchoolEventProgressPanel event={e} tenantHeaders={tenant.headers} tenantKey={tenant.key} />
      </Flex>
      {cancelling ? (
        <CancelSchoolEventDialog
          eventId={e.event_id}
          eventName={name}
          onClose={() => setCancelling(false)}
          onDone={() => {
            setCancelling(false);
            void q.refetch();
            navigate(eventPath(e, cross.crossTenant), { replace: true });
          }}
        />
      ) : null}
    </section>
  );
}
