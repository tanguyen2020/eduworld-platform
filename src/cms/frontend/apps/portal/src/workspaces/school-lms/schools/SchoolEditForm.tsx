// Edit the displayed information of a school (F052 T016, F052-AC-02, F052-AC-26): updateTenantSchool with If-Match
// (ETag of the last getTenantSchool, sent by the apiClient), active provinces only (listProvinces?status=active),
// SIS-owned data read-only. 412/422 shown at the field or as a Problem with trace id.
import { App, Button, Form, Input, InputNumber, Select, Space } from 'antd';
import { useTranslation } from 'react-i18next';
import { applyFormError, writeErrorKey } from '../common/errors';
import { FIELDS } from '../common/fieldSchemas';
import { localizedName } from '../common/format';
import { useFieldRules } from '../common/ui';
import { formValues, readOnlyFields, updateBody, type SchoolFormValues } from './schoolForm';
import { useActiveProvinces, useUpdateSchool, type School } from './useSchools';

const LEVELS: School['school_level'][] = ['primary', 'lower_secondary', 'multi_level'];
const FORM_FIELDS = ['name', 'name_vi', 'name_en', 'province_code', 'school_level', 'grade_min', 'grade_max', 'address'];

export function SchoolEditForm({ school, onDone }: { school: School; onDone: () => void }) {
  const { t, i18n } = useTranslation();
  const { message } = App.useApp();
  const [form] = Form.useForm<SchoolFormValues>();
  const provinces = useActiveProvinces();
  const update = useUpdateSchool(school.school_id);
  const field = useFieldRules();
  const locked = readOnlyFields(school);
  const submit = async (v: SchoolFormValues) => {
    const body = updateBody(school, v);
    if (!body) return onDone();
    try {
      await update.mutateAsync(body);
      message.success(t('portal.school_lms.common.saved'));
      onDone();
    } catch (e) {
      if (!applyFormError(form, e, FORM_FIELDS, t, i18n.exists.bind(i18n))) message.error(t(writeErrorKey(e)));
    }
  };
  return (
    <Form<SchoolFormValues> form={form} layout="vertical" initialValues={formValues(school)} onFinish={submit} data-testid="school-edit-form">
      <Form.Item name="name_vi" label={t('portal.school_lms.schools.nameVi')} rules={field.rules(FIELDS.schoolNameVi, true)}>
        <Input disabled={locked.has('name_vi')} {...field.input(FIELDS.schoolNameVi)} />
      </Form.Item>
      <Form.Item name="name_en" label={t('portal.school_lms.schools.nameEn')} rules={field.rules(FIELDS.schoolNameEn)}>
        <Input disabled={locked.has('name_en')} {...field.input(FIELDS.schoolNameEn)} />
      </Form.Item>
      <Form.Item name="province_code" label={t('portal.school_lms.schools.province')}>
        <Select
          disabled={locked.has('province_code')}
          showSearch
          optionFilterProp="label"
          loading={provinces.isFetching}
          options={(provinces.data ?? []).filter((p) => p.status === 'active').map((p) => ({ value: p.code, label: localizedName(p.name, i18n.language) }))}
        />
      </Form.Item>
      <Form.Item name="school_level" label={t('portal.school_lms.schools.level')} rules={[{ required: true }]}>
        <Select disabled={locked.has('school_level')} options={LEVELS.map((l) => ({ value: l, label: t(`portal.school_lms.schools.levels.${l}`) }))} />
      </Form.Item>
      <Space>
        <Form.Item name="grade_min" label={t('portal.school_lms.schools.gradeMin')} rules={[{ required: true }]}>
          <InputNumber min={1} max={9} disabled={locked.has('grade_min')} />
        </Form.Item>
        <Form.Item name="grade_max" label={t('portal.school_lms.schools.gradeMax')} rules={[{ required: true }]}>
          <InputNumber min={1} max={9} disabled={locked.has('grade_max')} />
        </Form.Item>
      </Space>
      <Form.Item name="address" label={t('portal.school_lms.schools.address')} rules={field.rules(FIELDS.schoolAddress)}>
        <Input.TextArea disabled={locked.has('address')} autoSize={{ minRows: 2, maxRows: 4 }} {...field.input(FIELDS.schoolAddress)} />
      </Form.Item>
      {locked.size ? <p>{t('portal.school_lms.schools.sisReadOnly')}</p> : null}
      <Space>
        <Button onClick={onDone}>{t('portal.school_lms.common.cancel')}</Button>
        <Button type="primary" htmlType="submit" loading={update.isPending}>
          {t('portal.school_lms.common.save')}
        </Button>
      </Space>
    </Form>
  );
}
