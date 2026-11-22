// School picker of the screens working on one school (classes, SIS, school settings, audit filter). Options come from
// listTenantSchools — a School Admin only receives the schools of their scope, a Tenant Admin all schools of the tenant
// (edge case "tenant unit có nhiều trường"); the only school is selected automatically.

import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { useSchoolList } from '../schools/useSchools';
import { localizedName } from './format';
import { SearchSelect } from '@eduworld/ops-ui';

export interface SchoolSelectProps {
  value: string | undefined;
  onChange: (schoolId: string | undefined) => void;
  allowClear?: boolean;
  /** Pick the first school when none is selected (screens that always need one). */
  autoSelect?: boolean;
  id?: string;
}

export function SchoolSelect({ value, onChange, allowClear = false, autoSelect = true, id }: SchoolSelectProps) {
  const { t, i18n } = useTranslation();
  const schools = useSchoolList();
  const first = schools.items[0]?.school_id;
  useEffect(() => {
    if (autoSelect && !value && first) onChange(first);
  }, [autoSelect, value, first, onChange]);
  return (
    <SearchSelect
      id={id}
      aria-label={t('portal.school_lms.common.school')}
      placeholder={t('portal.school_lms.common.pickSchool')}
      value={value}
      onChange={(v?: string) => onChange(v)}
      allowClear={allowClear}
      loading={schools.isFetching}
      showSearch
      optionFilterProp="label"
      popupMatchSelectWidth={false}
      options={schools.items.map((s) => ({ value: s.school_id, label: localizedName(s.name, i18n.language) }))}
      style={{ minWidth: '100%' }}
    />
  );
}
