// Screen of `menu.school_lms.school_events` (F081 T001, T035; ADMIN_PORTAL §3.15, ADR-0090): routes of "Sự kiện trường"
// under the catalog path /school-lms/school-events — list (index), create from a template (/new), detail with the progress
// (/:eventId) and edit before the start (/:eventId/edit). The menu, its path and permission gate come from the generated
// catalog (WorkspaceRoutes → 403 page without lms.school_event.write); buttons are wrapped in ActionGate.
import { Route, Routes } from 'react-router-dom';
import { NotFoundResult } from '../common/errors';
import { CreateSchoolEventWizard } from './CreateSchoolEventWizard';
import { EditSchoolEventPage } from './EditSchoolEventPage';
import { SchoolEventDetailPage } from './SchoolEventDetailPage';
import { SchoolEventListPage } from './SchoolEventListPage';

export function SchoolEventsScreen({ title }: { title: string }) {
  return (
    <Routes>
      <Route index element={<SchoolEventListPage title={title} />} />
      <Route path="new" element={<CreateSchoolEventWizard />} />
      <Route path=":eventId" element={<SchoolEventDetailPage />} />
      <Route path=":eventId/edit" element={<EditSchoolEventPage />} />
      <Route path="*" element={<NotFoundResult />} />
    </Routes>
  );
}
