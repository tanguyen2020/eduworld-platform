// Screen of `menu.school_lms.school_dashboard` (F056 T001, T045, T051; ADMIN_PORTAL §1.2, SCHOOL_LMS §8): the menu, its
// path and permissions (analytics.school.read / analytics.school.read, any-of) come from the generated catalog — the
// MenuGate shows the portal 403 / "tính năng tạm tắt" page and never mounts this screen (no dashboard request,
// F056-AC-20). Routes: overview (index), class (`classes/:classId`), student (`students/:studentId`), reference levels
// TT27/TT22 (`reference`). One DashboardQueue per screen limits concurrent queries (F056-AC-04). A System Admin works on
// the one tenant picked in the topbar (X-Tenant-Id).
import { DashboardQueueProvider } from '@eduworld/ops-ui/dashboard';
import { Route, Routes } from 'react-router-dom';
import { SingleTenantGate } from '../common/scope';
import { PageTitle } from '../common/ui';
import { ClassDashboardPage } from './class/ClassDashboardPage';
import { DASHBOARD_CONFIG } from './model/limits';
import { OverviewPage } from './overview/OverviewPage';
import { ReferenceLevelsPage } from './reports/ReferenceLevelsPage';
import { StudentProfilePage } from './student/StudentProfilePage';

export function SchoolDashboardScreen({ title }: { title: string }) {
  return (
    <section aria-labelledby="ew-menu-title">
      <PageTitle title={title} />
      <SingleTenantGate>
        {() => (
          <DashboardQueueProvider limit={DASHBOARD_CONFIG.dashboard_max_concurrent_per_user}>
            <Routes>
              <Route index element={<OverviewPage />} />
              <Route path="classes/:classId" element={<ClassDashboardPage />} />
              <Route path="students/:studentId" element={<StudentProfilePage />} />
              <Route path="reference" element={<ReferenceLevelsPage />} />
            </Routes>
          </DashboardQueueProvider>
        )}
      </SingleTenantGate>
    </section>
  );
}
