// "Tạo sự kiện" (F081 US1: F081-AC-01…AC-05, AC-06 target tenant, AC-19): step 1 picks a template among the templates
// assigned to the tenant (listEventTemplates, every page; none → an empty state asking to contact EduWorld, never a free
// form); System Admin first picks the ONE target tenant (TenantTargetField, never "all tenants"). Step 2 is the form of
// the template (SchoolEventForm). "Tạo" calls createSchoolEvent with an Idempotency-Key generated once for the intent and
// kept on retry (network error, double click, 503), then opens the new event. 422 errors go to their field (a flagged text
// shows a generic message only), 503 keeps the form, 409 says the concurrent limit is reached. Below 768 px: "Tác vụ này
// cần màn hình lớn hơn" (view only).
import { Button, Card, Empty, Flex, Form, List, Tag, Typography } from 'antd';
import { useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { ActionGate } from '../../../permissions/ActionGate';
import { ApiError } from '../../../shared/api/middleware/problem';
import { TENANT_HEADER } from '../../../shared/api/middleware/tenant';
import { useCrossTenantScope } from '../../../shared/tenancy/useCrossTenantScope';
import { MOBILE_QUERY } from '../../../shell/ResponsiveTable';
import { LargeScreenRequired } from '../../../shell/StatusPages';
import { TenantTargetField } from '../../../shell/TenantControls';
import { useMediaQuery } from '../../../shell/useMediaQuery';
import { ProblemMessage } from '../common/errors';
import { localizedName } from '../common/format';
import { PageTitle } from '../common/ui';
import { useCreateSchoolEvent, useEventTemplates, type EventTemplate, type SchoolEventCreate } from './api';
import { mapEventProblem } from './mapProblem';
import { SchoolEventForm, eventParams, type EventFormValues } from './SchoolEventForm';
import { eventPath } from './SchoolEventListPage';
import { ictIso } from './ui';

function TemplatePicker({ templates, loading, onPick }: { templates: EventTemplate[]; loading: boolean; onPick: (t: EventTemplate) => void }) {
  const { t, i18n } = useTranslation();
  if (!loading && templates.length === 0) {
    return (
      <Empty data-testid="no-templates" description={<Flex vertical gap="small"><Typography.Text strong>{t('portal.school_lms.schoolEvents.templates.empty')}</Typography.Text><Typography.Text type="secondary">{t('portal.school_lms.schoolEvents.templates.emptyHelp')}</Typography.Text></Flex>} />
    );
  }
  return (
    <List
      loading={loading && templates.length === 0}
      grid={{ gutter: 16, xs: 1, md: 2, xl: 3 }}
      dataSource={templates}
      renderItem={(x) => (
        <List.Item>
          <Card
            title={localizedName(x.name, i18n.language)}
            extra={<Tag>{t(`portal.school_lms.schoolEvents.templates.kind.${x.event_kind}`, { defaultValue: x.event_kind })}</Tag>}
            actions={[
              <Button key="pick" type="link" onClick={() => onPick(x)} aria-label={t('portal.school_lms.schoolEvents.templates.chooseNamed', { name: localizedName(x.name, i18n.language) })}>
                {t('portal.school_lms.schoolEvents.templates.choose')}
              </Button>,
            ]}
          >
            {x.description ? <Typography.Paragraph>{localizedName(x.description, i18n.language)}</Typography.Paragraph> : null}
            {x.max_duration_days ? <Typography.Text type="secondary">{t('portal.school_lms.schoolEvents.templates.maxDuration', { days: x.max_duration_days })}</Typography.Text> : null}
          </Card>
        </List.Item>
      )}
    />
  );
}

export function CreateSchoolEventWizard() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const wide = useMediaQuery(MOBILE_QUERY);
  const cross = useCrossTenantScope();
  const [targetForm] = Form.useForm<{ tenant_target: string }>();
  const target = Form.useWatch('tenant_target', targetForm);
  const headers: Record<string, string> | null = cross.crossTenant ? (target ? { [TENANT_HEADER]: target } : null) : {};
  const tenantKey = cross.crossTenant ? (target ?? 'none') : 'session';
  const templates = useEventTemplates(headers ?? {}, tenantKey, !!headers);
  const [template, setTemplate] = useState<EventTemplate | null>(null);
  const [form] = Form.useForm<EventFormValues>();
  const create = useCreateSchoolEvent();
  const [banner, setBanner] = useState<string | null>(null);
  const title = t('portal.school_lms.schoolEvents.createTitle');
  if (!wide) {
    return (
      <section>
        <PageTitle title={title} />
        <LargeScreenRequired />
      </section>
    );
  }
  const submit = (v: EventFormValues) => {
    if (!template || !headers) return;
    const body = { template_id: template.template_id, ...eventParams(v, template, ictIso) } as SchoolEventCreate;
    setBanner(null);
    create.mutate(
      { body, headers },
      {
        onSuccess: (ev) => navigate(eventPath({ event_id: ev.event_id, tenant_id: ev.tenant_id ?? target }, cross.crossTenant)),
        onError: (e) => {
          const m = mapEventProblem(e, 'create', t);
          if (m.fields.length) form.setFields(m.fields as Parameters<typeof form.setFields>[0]);
          setBanner(m.banner);
        },
      },
    );
  };
  return (
    <section>
      <PageTitle title={title} />
      <ActionGate actionKey="action.school_lms.school_events.edit">
        <Flex vertical gap="middle">
          {cross.crossTenant ? (
            <Form form={targetForm} layout="vertical" name="school_event_target" disabled={!!template}>
              <TenantTargetField />
            </Form>
          ) : null}
          {!headers ? (
            <Typography.Text type="secondary">{t('portal.school_lms.schoolEvents.templates.pickTenant')}</Typography.Text>
          ) : template ? (
            <SchoolEventForm
              form={form}
              template={template}
              tenantHeaders={headers}
              tenantKey={tenantKey}
              submitLabel={t('portal.school_lms.schoolEvents.form.submit')}
              submitting={create.isPending}
              onSubmit={submit}
              onBack={() => {
                setTemplate(null);
                setBanner(null);
              }}
              backLabel={t('portal.school_lms.schoolEvents.form.changeTemplate')}
              banner={banner ? <ProblemBanner messageKey={banner} error={create.error} /> : null}
            />
          ) : (
            <>
              <Typography.Title level={2}>{t('portal.school_lms.schoolEvents.templates.title')}</Typography.Title>
              {templates.error ? <ProblemMessage error={templates.error} /> : null}
              <TemplatePicker templates={templates.items} loading={templates.isFetching} onPick={setTemplate} />
            </>
          )}
        </Flex>
      </ActionGate>
    </section>
  );
}

/** Write error not on a field: the screen message plus the Problem code and trace_id (F081-AC-19). */
export function ProblemBanner({ messageKey, error, action }: { messageKey: string; error: unknown; action?: ReactNode }) {
  const { t } = useTranslation();
  const p = error instanceof ApiError ? error : null;
  return (
    <Flex vertical gap="small" role="alert" data-testid="event-error">
      <Typography.Text type="danger">{t(messageKey)}</Typography.Text>
      {p?.code ? (
        <Typography.Text type="secondary" copyable={p.traceId ? { text: p.traceId } : false}>
          {t('portal.school_lms.schoolEvents.errors.code', { code: p.code, traceId: p.traceId ?? '—' })}
        </Typography.Text>
      ) : null}
      {action}
    </Flex>
  );
}
