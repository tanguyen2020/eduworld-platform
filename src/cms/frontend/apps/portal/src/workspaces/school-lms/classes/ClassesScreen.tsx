// Screen of `menu.school_lms.classes` (F052 T017, T018): tabs "Lớp" | "Học sinh chưa có lớp"; a class's students at the
// sub-route `:schoolId/:classId`.
import { Tabs } from 'antd';
import { useTranslation } from 'react-i18next';
import { Route, Routes } from 'react-router-dom';
import { SingleTenantGate } from '../common/scope';
import { PageTitle } from '../common/ui';
import { UnassignedStudentsPage } from '../students/UnassignedStudentsPage';
import { ClassListPage } from './ClassListPage';
import { ClassStudentsPage } from './ClassStudentsPage';

function ClassesHome() {
  const { t } = useTranslation();
  return (
    <Tabs
      items={[
        { key: 'classes', label: t('portal.school_lms.classes.tab'), children: <ClassListPage /> },
        { key: 'unassigned', label: t('portal.school_lms.unassigned.tab'), children: <UnassignedStudentsPage /> },
      ]}
    />
  );
}

export function ClassesScreen({ title }: { title: string }) {
  return (
    <section aria-labelledby="ew-menu-title">
      <PageTitle title={title} />
      <SingleTenantGate>
        {() => (
          <Routes>
            <Route index element={<ClassesHome />} />
            <Route path=":schoolId/:classId" element={<ClassStudentsPage />} />
          </Routes>
        )}
      </SingleTenantGate>
    </section>
  );
}
