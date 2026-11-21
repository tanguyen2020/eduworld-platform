// Screen of `menu.school_lms.sis_sync` (F052 T025, T026): school picker, tabs "Nhập CSV" | "Xung đột đồng bộ". The tab
// "Học sinh chưa có lớp" (listUnassignedStudents) is gone with the class concept (ADR-0095).
import { Form, Space, Tabs } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { SchoolSelect } from '../common/SchoolSelect';
import { SingleTenantGate } from '../common/scope';
import { PageTitle } from '../common/ui';
import { ReconciliationIssuesPage } from './ReconciliationIssuesPage';
import { SisImportPage } from './SisImportPage';

function SisHome() {
  const { t } = useTranslation();
  const [schoolId, setSchoolId] = useState<string | undefined>();
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Form.Item label={t('portal.school_lms.common.school')}>
        <SchoolSelect value={schoolId} onChange={setSchoolId} />
      </Form.Item>
      {schoolId ? (
        <Tabs
          destroyOnHidden
          items={[
            { key: 'import', label: t('portal.school_lms.sis.tab'), children: <SisImportPage schoolId={schoolId} /> },
            { key: 'issues', label: t('portal.school_lms.recon.tab'), children: <ReconciliationIssuesPage schoolId={schoolId} /> },
          ]}
        />
      ) : null}
    </Space>
  );
}

export function SisScreen({ title }: { title: string }) {
  return (
    <section aria-labelledby="ew-menu-title">
      <PageTitle title={title} />
      <SingleTenantGate>{() => <SisHome />}</SingleTenantGate>
    </section>
  );
}
