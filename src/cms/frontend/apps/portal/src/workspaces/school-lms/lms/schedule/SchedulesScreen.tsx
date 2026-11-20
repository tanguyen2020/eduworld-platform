// Screen of `menu.school_lms.schedules` (F055 T035, F055-AC-17): month calendar or week list of listScheduleItems for
// the visible range (optionally one course); with lms.schedule.write (catalog action schedules.edit) create / edit /
// delete an item — kind, date, period, note within the contract limits. Delete = soft-delete on the server.
import { Button, Calendar, DatePicker, Flex, Form, Input, InputNumber, List, Modal, Segmented, Select, Space, Tag, Typography } from 'antd';
import dayjs, { type Dayjs } from 'dayjs';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../../permissions/ActionGate';
import { SingleTenantGate } from '../../common/scope';
import { PageTitle, useFieldRules } from '../../common/ui';
import { useCourses } from '../api/courses';
import { useDeleteScheduleItem, useSaveScheduleItem, useScheduleItems, type ScheduleItem, type ScheduleItemInput } from '../api/schedules';
import { courseLabel } from '../assignments/wizard/StepTargets';
import { LMS_ENUMS, LMS_FIELDS, LMS_LIMITS } from '../model/fieldSchemas';
import { LmsProblem, useActionEnabled } from '../LmsUi';

const DAY = 'YYYY-MM-DD';

interface ItemForm {
  course_id: string;
  date: Dayjs;
  period?: number;
  kind: ScheduleItemInput['kind'];
  note?: string;
}

function ItemDialog({ item, date, courseId, onClose }: { item?: ScheduleItem; date: Dayjs; courseId?: string; onClose: () => void }) {
  const { t, i18n } = useTranslation();
  const field = useFieldRules();
  const [form] = Form.useForm<ItemForm>();
  const courses = useCourses({ status: 'active' });
  const save = useSaveScheduleItem();
  const del = useDeleteScheduleItem();
  return (
    <Modal
      open
      title={item ? t('lms.schedule.editTitle') : t('lms.schedule.newTitle')}
      okText={t('portal.school_lms.common.save')}
      cancelText={t('portal.school_lms.common.cancel')}
      confirmLoading={save.isPending}
      destroyOnHidden
      onCancel={onClose}
      onOk={async () => {
        const v = await form.validateFields().catch(() => null);
        if (!v) return;
        const body: ScheduleItemInput = { course_id: v.course_id, date: v.date.format(DAY), kind: v.kind, ...(v.period ? { period: v.period } : {}), ...(v.note?.trim() ? { note: v.note.trim() } : {}) };
        save.mutate({ item, body }, { onSuccess: onClose });
      }}
      footer={(orig) => (
        <Space wrap>
          {item ? (
            <Button
              danger
              loading={del.isPending}
              onClick={() =>
                Modal.confirm({
                  title: t('lms.schedule.deleteConfirm'),
                  okText: t('portal.school_lms.common.delete'),
                  cancelText: t('portal.school_lms.common.cancel'),
                  onOk: () => del.mutate(item.schedule_item_id, { onSuccess: onClose }),
                })
              }
            >
              {t('portal.school_lms.common.delete')}
            </Button>
          ) : null}
          {orig}
        </Space>
      )}
    >
      <LmsProblem error={save.error ?? del.error} />
      <Form form={form} layout="vertical" initialValues={{ course_id: item?.course_id ?? courseId, date: item ? dayjs(item.date) : date, period: item?.period, kind: item?.kind ?? 'lesson', note: item?.note }}>
        <Form.Item name="course_id" label={t('lms.wizard.course')} rules={[{ required: true, message: t('lms.form.required') }]}>
          <Select aria-label={t('lms.wizard.course')} options={courses.items.map((c) => ({ value: c.course_id, label: courseLabel(c, i18n.language) }))} />
        </Form.Item>
        <Form.Item name="date" label={t('lms.schedule.date')} rules={[{ required: true, message: t('lms.form.required') }]}>
          <DatePicker aria-label={t('lms.schedule.date')} />
        </Form.Item>
        <Form.Item name="kind" label={t('lms.schedule.kind')}>
          <Select aria-label={t('lms.schedule.kind')} options={LMS_ENUMS.scheduleKind.values.map((k) => ({ value: k, label: t(`lms.scheduleKind.${k}`) }))} />
        </Form.Item>
        <Form.Item name="period" label={t('lms.schedule.period')}>
          <InputNumber min={LMS_LIMITS.schedulePeriod.minimum} max={LMS_LIMITS.schedulePeriod.maximum} precision={0} />
        </Form.Item>
        <Form.Item name="note" label={t('lms.schedule.note')} rules={field.rules(LMS_FIELDS.scheduleNote)}>
          <Input.TextArea autoSize={{ minRows: 2, maxRows: 5 }} {...field.input(LMS_FIELDS.scheduleNote)} />
        </Form.Item>
      </Form>
    </Modal>
  );
}

function ScheduleBoard() {
  const { t, i18n } = useTranslation();
  const [view, setView] = useState<'month' | 'week'>('month');
  const [anchor, setAnchor] = useState(dayjs());
  const [courseId, setCourseId] = useState<string | undefined>();
  const [dialog, setDialog] = useState<{ item?: ScheduleItem; date: Dayjs } | null>(null);
  const canEdit = useActionEnabled('action.school_lms.schedules.edit');
  const courses = useCourses({ status: 'active' });
  const from = (view === 'month' ? anchor.startOf('month') : anchor.startOf('week')).format(DAY);
  const to = (view === 'month' ? anchor.endOf('month') : anchor.endOf('week')).format(DAY);
  const items = useScheduleItems({ from, to, courseId });
  const byDay = (d: Dayjs) => items.items.filter((i) => i.date === d.format(DAY));
  const chip = (i: ScheduleItem) => (
    <Tag key={i.schedule_item_id} data-testid="schedule-item" onClick={canEdit ? () => setDialog({ item: i, date: dayjs(i.date) }) : undefined}>
      {t(`lms.scheduleKind.${i.kind}`)}
      {i.note ? ` · ${i.note}` : ''}
    </Tag>
  );
  const week = Array.from({ length: 7 }, (_, k) => anchor.startOf('week').add(k, 'day'));
  return (
    <Flex vertical gap="middle">
      <Flex wrap="wrap" gap="middle" align="center">
        <Segmented<'month' | 'week'> value={view} onChange={setView} options={[{ value: 'month', label: t('lms.schedule.month') }, { value: 'week', label: t('lms.schedule.week') }]} />
        <Select
          allowClear
          aria-label={t('lms.wizard.course')}
          placeholder={t('lms.wizard.course')}
          value={courseId}
          onChange={setCourseId}
          options={courses.items.map((c) => ({ value: c.course_id, label: courseLabel(c, i18n.language) }))}
        />
        <ActionGate actionKey="action.school_lms.schedules.edit">
          <Button type="primary" onClick={() => setDialog({ date: anchor })}>
            {t('lms.schedule.add')}
          </Button>
        </ActionGate>
      </Flex>
      <LmsProblem error={items.error} />
      {view === 'month' ? (
        <Calendar value={anchor} onPanelChange={(d) => setAnchor(d)} onSelect={(d) => setAnchor(d)} cellRender={(d, info) => (info.type === 'date' ? <Flex vertical>{byDay(d).map(chip)}</Flex> : null)} />
      ) : (
        <List
          data-testid="schedule-week"
          header={
            <Space>
              <Button onClick={() => setAnchor(anchor.subtract(1, 'week'))}>{t('lms.schedule.prevWeek')}</Button>
              <Typography.Text>{`${week[0]!.format('DD/MM')} – ${week[6]!.format('DD/MM/YYYY')}`}</Typography.Text>
              <Button onClick={() => setAnchor(anchor.add(1, 'week'))}>{t('lms.schedule.nextWeek')}</Button>
            </Space>
          }
          dataSource={week}
          renderItem={(d) => (
            <List.Item>
              <Space wrap>
                <Typography.Text strong>{d.format('dd DD/MM')}</Typography.Text>
                {byDay(d).map(chip)}
              </Space>
            </List.Item>
          )}
        />
      )}
      {dialog ? <ItemDialog item={dialog.item} date={dialog.date} courseId={courseId} onClose={() => setDialog(null)} /> : null}
    </Flex>
  );
}

export function SchedulesScreen({ title }: { title: string }) {
  return (
    <section aria-labelledby="ew-menu-title">
      <PageTitle title={title} />
      <SingleTenantGate>{() => <ScheduleBoard />}</SingleTenantGate>
    </section>
  );
}
