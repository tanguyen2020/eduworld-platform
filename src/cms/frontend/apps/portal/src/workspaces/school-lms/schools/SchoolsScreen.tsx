// Screen of `menu.school_lms.schools` (F052 T016): list and `:schoolId` detail as sub-routes of the catalog path.
import { Route, Routes } from 'react-router-dom';
import { SingleTenantGate } from '../common/scope';
import { PageTitle } from '../common/ui';
import { SchoolDetailPage } from './SchoolDetailPage';
import { SchoolListPage } from './SchoolListPage';

export function SchoolsScreen({ title }: { title: string }) {
  return (
    <section aria-labelledby="ew-menu-title">
      <PageTitle title={title} />
      <SingleTenantGate>
        {() => (
          <Routes>
            <Route index element={<SchoolListPage />} />
            <Route path=":schoolId" element={<SchoolDetailPage />} />
          </Routes>
        )}
      </SingleTenantGate>
    </section>
  );
}
