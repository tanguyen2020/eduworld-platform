// "Gửi thông báo trường" (F122-AC-05, MODERATION §6.4): one school among the `school_ids` of the case, only the related
// students of that school (the school of each student comes from the case messages, `sender_school_id`; ≤ 20), a summary
// from the template (10–1000 characters, "không chép nội dung chat") → notifySchoolForCase with Idempotency-Key. A student
// is shown by nickname or id only (R16). The server checks everything again.
import { Alert, Button, Checkbox, Drawer, Flex, Form, Input, Typography } from 'antd';
import { openapiInputProps, openapiRules, OPS_UI_NS, SearchSelect } from '@eduworld/ops-ui';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { LIMITS, MAX_NOTIFIED_STUDENTS } from '../../../shared/chatSafety/ui';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { useCaseMessages, useNotifySchool, type ModerationCaseDetail } from './api';

interface NotifyForm {
  school_id: string;
  student_ids: string[];
  summary: string;
}

export function NotifySchoolDrawer({ c, onClose }: { c: ModerationCaseDetail; onClose: () => void }) {
  const { t } = useTranslation();
  const ops = useTranslation(OPS_UI_NS);
  const [form] = Form.useForm<NotifyForm>();
  const school = Form.useWatch('school_id', form);
  const messages = useCaseMessages(c.case_id, '');
  const notify = useNotifySchool(c);
  /** student → { school, nickname } from the case messages. */
  const known = useMemo(() => {
    const m = new Map<string, { school?: string; nickname?: string }>();
    for (const x of messages.rows) if (!m.has(x.sender_id)) m.set(x.sender_id, { school: x.sender_school_id, nickname: x.sender_nickname });
    return m;
  }, [messages.rows]);
  const students = (c.subject_user_ids ?? []).filter((id) => school && known.get(id)?.school === school);
  return (
    <Drawer open title={t('chatSafety.notify.title')} onClose={onClose} width="min(32rem, 100vw)" destroyOnHidden>
      <Form
        form={form}
        layout="vertical"
        name="notify_school"
        onFinish={(v) => notify.mutate({ school_id: v.school_id, student_ids: v.student_ids, summary: v.summary.trim() }, { onSuccess: onClose })}
      >
        <Form.Item name="school_id" label={t('chatSafety.notify.school')} rules={[{ required: true, message: t('chatSafety.notify.schoolRequired') }]}>
          <SearchSelect
            aria-label={t('chatSafety.notify.school')}
            options={(c.school_ids ?? []).map((id) => ({ value: id, label: id }))}
            onChange={() => form.setFieldValue('student_ids', [])}
          />
        </Form.Item>
        <Form.Item
          name="student_ids"
          label={t('chatSafety.notify.students', { max: MAX_NOTIFIED_STUDENTS })}
          rules={[{ required: true, type: 'array', min: 1, message: t('chatSafety.notify.studentsRequired') }, { type: 'array', max: MAX_NOTIFIED_STUDENTS }]}
        >
          {students.length ? (
            <Checkbox.Group>
              <Flex vertical>
                {students.slice(0, MAX_NOTIFIED_STUDENTS).map((id) => (
                  <Checkbox key={id} value={id}>
                    {known.get(id)?.nickname ?? id}
                  </Checkbox>
                ))}
              </Flex>
            </Checkbox.Group>
          ) : (
            <Typography.Text type="secondary">{school ? t('chatSafety.notify.noStudent') : t('chatSafety.notify.pickSchoolFirst')}</Typography.Text>
          )}
        </Form.Item>
        <Alert type="info" showIcon message={t('chatSafety.notify.noChatContent')} />
        <Form.Item
          name="summary"
          label={t('chatSafety.notify.summary')}
          initialValue={t('chatSafety.notify.template')}
          rules={openapiRules(LIMITS.notificationSummary, { required: true, t: ops.t })}
        >
          <Input.TextArea autoSize={{ minRows: 4, maxRows: 10 }} {...openapiInputProps(LIMITS.notificationSummary)} />
        </Form.Item>
        {notify.error ? <ProblemMessage error={notify.error} /> : null}
        <Button type="primary" htmlType="submit" loading={notify.isPending}>
          {t('chatSafety.notify.send')}
        </Button>
      </Form>
    </Drawer>
  );
}
