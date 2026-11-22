// Scope target of a UserRole (F052-AC-16, ADR-0072): a school (listTenantSchools — a School Admin only sees their
// own) or a subject (listSubjects of the tenant). "Đơn vị" has no target. No class scope exists.
import { Form } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { qk, useApi } from '../common/api';
import { localizedName } from '../common/format';
import { useSchoolLmsScope } from '../common/scope';
import { useSchoolList } from '../schools/useSchools';
import type { GrantMode, GrantScope } from './canGrant';
import { SearchSelect } from '@eduworld/ops-ui';

function useSubjects(enabled: boolean) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  return useQuery({
    queryKey: qk('subjects', scope),
    queryFn: async () => (await api.call(api.client.GET('/learning/subjects', { headers: scope.headers }))).items,
    enabled: scope.ready && enabled,
    staleTime: 10 * 60_000,
  });
}

export function ScopeTargetField({ scopeType, mode }: { scopeType: GrantScope | undefined; mode: GrantMode }) {
  const { t, i18n } = useTranslation();
  const schools = useSchoolList();
  const subjects = useSubjects(scopeType === 'subject');
  if (!scopeType || scopeType === 'tenant') return null;
  const schoolOptions = schools.items
    .filter((s) => mode.kind === 'tenant' || mode.schoolIds.includes(s.school_id))
    .map((s) => ({ value: s.school_id, label: localizedName(s.name, i18n.language) }));
  const subjectOptions = (subjects.data ?? []).map((s) => ({ value: s.subject_id, label: s.name.vi ?? s.code }));
  return (
    <Form.Item name="scope_id" label={t(`portal.school_lms.userRoles.target.${scopeType}`)} rules={[{ required: true, message: t('portal.school_lms.userRoles.targetRequired') }]}>
      <SearchSelect showSearch optionFilterProp="label" loading={scopeType === 'school' ? schools.isFetching : subjects.isFetching} options={scopeType === 'school' ? schoolOptions : subjectOptions} />
    </Form.Item>
  );
}
