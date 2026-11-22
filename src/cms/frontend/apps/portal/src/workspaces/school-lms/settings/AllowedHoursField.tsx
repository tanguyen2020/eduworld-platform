// Editor of `AllowedHours.windows` (days + from/to HH:MM, ≤ 14 windows) used by tenant and school settings. Same rules
// for every student — no option per grade/class (AGENTS R15).
import { Button, Form, Input, Space } from 'antd';
import { useTranslation } from 'react-i18next';
import { HHMM } from './curfewCovers';
import { SearchSelect } from '@eduworld/ops-ui';

const DAYS = ['mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun'] as const;
const MAX_WINDOWS = 14;

export function AllowedHoursField({ disabled }: { disabled?: boolean }) {
  const { t } = useTranslation();
  const time = [{ pattern: HHMM, message: t('portal.school_lms.settings.timeFormat') }, { required: true, message: t('portal.school_lms.settings.timeFormat') }];
  return (
    <Form.List name={['allowed_hours', 'windows']}>
      {(fields, { add, remove }) => (
        <Space direction="vertical" style={{ width: '100%' }}>
          {fields.map((f) => (
            <Space key={f.key} wrap align="start">
              <Form.Item name={[f.name, 'days']} rules={[{ required: true, type: 'array', min: 1, message: t('portal.school_lms.settings.daysRequired') }]}>
                <SearchSelect mode="multiple" disabled={disabled} aria-label={t('portal.school_lms.settings.days')} options={DAYS.map((d) => ({ value: d, label: t(`portal.school_lms.settings.weekdays.${d}`) }))} />
              </Form.Item>
              <Form.Item name={[f.name, 'from']} rules={time}>
                <Input disabled={disabled} maxLength={5} aria-label={t('portal.school_lms.settings.from')} placeholder={t('portal.school_lms.settings.timeHint')} />
              </Form.Item>
              <Form.Item name={[f.name, 'to']} rules={time}>
                <Input disabled={disabled} maxLength={5} aria-label={t('portal.school_lms.settings.to')} placeholder={t('portal.school_lms.settings.timeHint')} />
              </Form.Item>
              {disabled ? null : <Button onClick={() => remove(f.name)}>{t('portal.school_lms.common.remove')}</Button>}
            </Space>
          ))}
          {disabled || fields.length >= MAX_WINDOWS ? null : <Button onClick={() => add({ days: ['mon', 'tue', 'wed', 'thu', 'fri'], from: '07:00', to: '17:00' })}>{t('portal.school_lms.settings.addWindow')}</Button>}
        </Space>
      )}
    </Form.List>
  );
}
