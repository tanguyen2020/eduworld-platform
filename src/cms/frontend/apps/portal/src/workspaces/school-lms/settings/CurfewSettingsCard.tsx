// "Giờ giới nghiêm" of a school (F052 T057, F052-AC-29, F052-AC-30, ADR-0078, CHILD_SAFETY §4.1): platform curfew
// read-only ("do EduWorld đặt, áp cho game và chat"), the school window (empty = platform only) and the effective curfew
// returned by the server (never computed here). Editing needs action.school_lms.settings.edit_school and is absent for
// a school of the `common` tenant (virtual school). Early check: the school window must contain the whole platform
// window (else error at the field and Save locked); reason required; "Bỏ khung trường" sends `curfew_window: null`.
// 422 errors[curfew_window.*] → that field; 412 → reload the saved values. One rule for every student: no grade option.
import { Alert, App, Button, Card, Descriptions, Form, Input, Space, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ApiError } from '../../../shared/api/middleware/problem';
import { applyFormError, writeErrorKey } from '../common/errors';
import { FIELDS } from '../common/fieldSchemas';
import { useFieldRules } from '../common/ui';
import { crossesMidnight, curfewCovers, HHMM, type CurfewWindow } from './curfewCovers';
import { useUpdateSchoolSettings, type SchoolSettings } from './useSettings';

export function CurfewText({ w }: { w: CurfewWindow | null | undefined }) {
  const { t } = useTranslation();
  if (!w) return <>{t('portal.school_lms.curfew.none')}</>;
  return <>{t(crossesMidnight(w) ? 'portal.school_lms.curfew.rangeNextDay' : 'portal.school_lms.curfew.range', { start: w.start, end: w.end })}</>;
}

interface CurfewForm {
  start: string;
  end: string;
  reason: string;
}

export function CurfewSettingsCard({ schoolId, settings, isCommon, onReload }: { schoolId: string; settings: SchoolSettings; isCommon: boolean; onReload: () => void }) {
  const { t, i18n } = useTranslation();
  const { message } = App.useApp();
  const [form] = Form.useForm<CurfewForm>();
  const update = useUpdateSchoolSettings(schoolId);
  const field = useFieldRules();
  const [editing, setEditing] = useState(false);
  const start = Form.useWatch('start', form) ?? '';
  const end = Form.useWatch('end', form) ?? '';
  const platform = settings.platform_curfew;
  const complete = HHMM.test(start) && HHMM.test(end);
  const notCovering = complete && !!platform && !curfewCovers({ start, end }, platform);

  const send = async (curfew_window: CurfewWindow | null) => {
    const v = await form.validateFields(['reason']).catch(() => null);
    if (!v) return;
    try {
      await update.mutateAsync({ curfew_window, reason: v.reason.trim() });
      message.success(t('portal.school_lms.common.saved'));
      setEditing(false);
    } catch (e) {
      if (e instanceof ApiError && e.code === 'VERSION_MISMATCH') {
        message.warning(t('portal.school_lms.curfew.reloaded'));
        onReload();
        return;
      }
      const remapped = e instanceof ApiError ? Object.fromEntries(Object.entries(e.fieldErrors).map(([k, v2]) => [k === 'curfew_window' ? 'start' : k.replace(/^curfew_window\./, ''), v2])) : {};
      if (e instanceof ApiError && Object.keys(remapped).length) {
        const fake = new ApiError({ status: e.status, code: e.code, i18nKey: e.i18nKey, fieldErrors: remapped, traceId: e.traceId });
        if (applyFormError(form, fake, ['start', 'end', 'reason'], t, i18n.exists.bind(i18n))) return;
      }
      message.error(t(writeErrorKey(e)));
    }
  };

  return (
    <Card title={t('portal.school_lms.curfew.title')} data-testid="curfew-card">
      <Descriptions column={1} size="small">
        <Descriptions.Item label={t('portal.school_lms.curfew.platform')}>
          <Space direction="vertical" size={0}>
            <span data-testid="platform-curfew">
              <CurfewText w={platform} />
            </span>
            <Typography.Text type="secondary">{t('portal.school_lms.curfew.platformNote')}</Typography.Text>
          </Space>
        </Descriptions.Item>
        {isCommon ? null : (
          <Descriptions.Item label={t('portal.school_lms.curfew.school')}>
            <span data-testid="school-curfew">
              {settings.curfew_window ? <CurfewText w={settings.curfew_window} /> : t('portal.school_lms.curfew.schoolEmpty')}
            </span>
          </Descriptions.Item>
        )}
        <Descriptions.Item label={t('portal.school_lms.curfew.effective')}>
          <span data-testid="effective-curfew">
            <CurfewText w={isCommon ? platform : (settings.effective_curfew ?? platform)} />
          </span>
        </Descriptions.Item>
      </Descriptions>
      {isCommon ? null : (
        <ActionGate actionKey="action.school_lms.settings.edit_school">
          {editing ? (
            <Form<CurfewForm> form={form} layout="vertical" initialValues={{ start: settings.curfew_window?.start ?? '', end: settings.curfew_window?.end ?? '', reason: '' }} data-testid="curfew-form">
              <Space wrap align="start">
                <Form.Item name="start" label={t('portal.school_lms.curfew.start')} rules={[{ required: true, pattern: HHMM, message: t('portal.school_lms.settings.timeFormat') }]} validateStatus={notCovering ? 'error' : undefined} help={notCovering ? t('portal.school_lms.curfew.mustCover') : undefined}>
                  <Input maxLength={5} placeholder={t('portal.school_lms.settings.timeHint')} />
                </Form.Item>
                <Form.Item name="end" label={t('portal.school_lms.curfew.end')} rules={[{ required: true, pattern: HHMM, message: t('portal.school_lms.settings.timeFormat') }]}>
                  <Input maxLength={5} placeholder={t('portal.school_lms.settings.timeHint')} />
                </Form.Item>
              </Space>
              <Form.Item name="reason" label={t('portal.school_lms.common.reason')} rules={field.rules(FIELDS.schoolSettingsReason, true)}>
                <Input.TextArea autoSize={{ minRows: 2, maxRows: 4 }} {...field.input(FIELDS.schoolSettingsReason)} />
              </Form.Item>
              {notCovering ? <Alert type="error" showIcon role="alert" message={t('portal.school_lms.curfew.mustCover')} /> : null}
              <Space wrap>
                <Button onClick={() => setEditing(false)}>{t('portal.school_lms.common.cancel')}</Button>
                {settings.curfew_window ? (
                  <Button danger loading={update.isPending} onClick={() => void send(null)}>
                    {t('portal.school_lms.curfew.clear')}
                  </Button>
                ) : null}
                <Button type="primary" disabled={!complete || notCovering} loading={update.isPending} onClick={() => void send({ start, end })}>
                  {t('portal.school_lms.common.save')}
                </Button>
              </Space>
            </Form>
          ) : (
            <Button onClick={() => setEditing(true)}>{t('portal.school_lms.curfew.edit')}</Button>
          )}
        </ActionGate>
      )}
    </Card>
  );
}
