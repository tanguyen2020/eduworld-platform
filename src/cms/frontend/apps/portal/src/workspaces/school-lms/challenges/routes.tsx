// Screen of `menu.school_lms.challenges` "Thử thách khối / giải trường" (F108 US3, T028; FR-LMS-004): list (index),
// create wizard (`new`), detail (`:challengeId`). The menu, its path and its permission (lms.challenge.write) come from
// the generated catalog (ADR-0090) — this file only renders the routes under it. A System Admin works on the one target
// tenant picked in the topbar (F108-AC-16): the wizard and the detail are keyed by that tenant, so switching it resets
// them and reloads the data of the new tenant.
import { Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { Route, Routes } from 'react-router-dom';
import { SingleTenantGate } from '../common/scope';
import { PageTitle } from '../common/ui';
import { ChallengeDetailPage } from './pages/ChallengeDetailPage';
import { ChallengeListPage } from './pages/ChallengeListPage';
import { ChallengeWizard } from './wizard/ChallengeWizard';

export function ChallengesScreen({ title }: { title: string }) {
  const { t } = useTranslation();
  return (
    <section aria-labelledby="ew-menu-title">
      <PageTitle title={title} />
      <Routes>
        <Route index element={<ChallengeListPage />} />
        <Route
          path="new"
          element={
            <SingleTenantGate>
              {(scope) => (
                <>
                  <Typography.Title level={2}>{t('lms.challenges.wizard.heading')}</Typography.Title>
                  <ChallengeWizard key={scope.key} />
                </>
              )}
            </SingleTenantGate>
          }
        />
        <Route path=":challengeId" element={<SingleTenantGate>{(scope) => <ChallengeDetailPage key={scope.key} />}</SingleTenantGate>} />
      </Routes>
    </section>
  );
}
