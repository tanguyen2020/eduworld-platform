// "Sửa sự kiện" (F081-AC-07, AC-19): only while the event is `scheduled` (otherwise no form); the form of its template
// (allowed_params — a template unassigned since keeps name and dates only), updateSchoolEvent with If-Match = the version
// read; 412 VERSION_MISMATCH or 409 CONFLICT → "dữ liệu đã thay đổi" with a reload that re-reads the event and refills
// the form. Below 768 px: "Tác vụ này cần màn hình lớn hơn".
import { Alert, Button, Form } from 'antd';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate, useParams } from 'react-router-dom';
import { ActionGate } from '../../../permissions/ActionGate';
import { useCrossTenantScope } from '../../../shared/tenancy/useCrossTenantScope';
import { MOBILE_QUERY } from '../../../shell/ResponsiveTable';
import { LargeScreenRequired } from '../../../shell/StatusPages';
import { useMediaQuery } from '../../../shell/useMediaQuery';
import { NotFoundResult, ProblemMessage } from '../common/errors';
import { localizedName } from '../common/format';
import { PageTitle } from '../common/ui';
import { useEventTemplates, useEventTenant, useSchoolEvent, useUpdateSchoolEvent, type EventTemplate, type SchoolEvent, type SchoolEventUpdate } from './api';
import { ProblemBanner } from './CreateSchoolEventWizard';
import { mapEventProblem } from './mapProblem';
import { SchoolEventForm, eventParams, type EventFormValues } from './SchoolEventForm';
import { eventPath } from './SchoolEventListPage';
import { ictDayjs, ictIso, isEditable, isHidden, isStale } from './ui';

/** The event as form values. */
export function formValues(e: SchoolEvent): EventFormValues {
  return {
    name: { vi: e.name.vi, ...(e.name.en ? { en: e.name.en } : {}) },
    description: e.description ? { vi: e.description.vi, ...(e.description.en ? { en: e.description.en } : {}) } : undefined,
    starts_at: ictDayjs(e.starts_at),
    ends_at: ictDayjs(e.ends_at),
    audience: { school_ids: e.audience?.school_ids ?? [], grade_levels: e.audience?.grade_levels ?? [] },
    community_goal_target: e.community_goal_target ?? null,
    quest_ids: e.quest_ids ?? [],
    challenge_ids: e.challenge_ids ?? [],
    reward_option_id: e.reward_option_id,
  };
}

/** A template no longer assigned: the event keeps running; only name and dates stay editable. */
const fallbackTemplate = (e: SchoolEvent): EventTemplate => ({ template_id: e.template_id, code: '', name: e.name as EventTemplate['name'], event_kind: 'school', allowed_params: [] });

export function EditSchoolEventPage() {
  const { t, i18n } = useTranslation();
  const { eventId } = useParams();
  const navigate = useNavigate();
  const wide = useMediaQuery(MOBILE_QUERY);
  const cross = useCrossTenantScope();
  const tenant = useEventTenant();
  const q = useSchoolEvent(eventId);
  const templates = useEventTemplates(tenant.headers, tenant.key, tenant.ready);
  const update = useUpdateSchoolEvent(eventId ?? '');
  const [form] = Form.useForm<EventFormValues>();
  const [banner, setBanner] = useState<string | null>(null);
  const e = q.data;
  useEffect(() => {
    if (e) form.setFieldsValue(formValues(e));
  }, [e, form]);
  if (q.error) return isHidden(q.error) ? <NotFoundResult /> : <ProblemMessage error={q.error} />;
  if (!e) return null;
  const title = t('portal.school_lms.schoolEvents.edit.title', { name: localizedName(e.name, i18n.language) });
  if (!wide) {
    return (
      <section>
        <PageTitle title={title} />
        <LargeScreenRequired />
      </section>
    );
  }
  if (!isEditable(e.status)) {
    return (
      <section>
        <PageTitle title={title} />
        <Alert type="info" showIcon message={t('portal.school_lms.schoolEvents.edit.notEditable')} />
      </section>
    );
  }
  const template = templates.items.find((x) => x.template_id === e.template_id) ?? fallbackTemplate(e);
  const reload = async () => {
    setBanner(null);
    update.reset();
    const r = await q.refetch();
    if (r.data) form.setFieldsValue(formValues(r.data));
  };
  const submit = (v: EventFormValues) => {
    setBanner(null);
    update.mutate(
      { body: eventParams(v, template, ictIso) as SchoolEventUpdate, version: e.version },
      {
        onSuccess: (ev) => navigate(eventPath({ event_id: ev.event_id, tenant_id: ev.tenant_id ?? e.tenant_id }, cross.crossTenant)),
        onError: (err) => {
          const m = mapEventProblem(err, 'update', t);
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
        <SchoolEventForm
          form={form}
          template={template}
          tenantHeaders={tenant.headers}
          tenantKey={tenant.key}
          submitLabel={t('portal.school_lms.schoolEvents.form.save')}
          submitting={update.isPending}
          onSubmit={submit}
          onBack={() => navigate(eventPath(e, cross.crossTenant))}
          backLabel={t('portal.school_lms.common.back')}
          banner={
            banner ? (
              <ProblemBanner
                messageKey={banner}
                error={update.error}
                action={isStale(update.error) ? <Button onClick={() => void reload()}>{t('portal.school_lms.schoolEvents.errors.reload')}</Button> : undefined}
              />
            ) : null
          }
        />
      </ActionGate>
    </section>
  );
}
