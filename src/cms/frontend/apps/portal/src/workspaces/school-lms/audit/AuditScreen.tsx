// Screen of `menu.school_lms.audit_logs` (F052 T046).
import { SingleTenantGate } from '../common/scope';
import { PageTitle } from '../common/ui';
import { AuditLogPage } from './AuditLogPage';

export function AuditScreen({ title }: { title: string }) {
  return (
    <section aria-labelledby="ew-menu-title">
      <PageTitle title={title} />
      <SingleTenantGate>{() => <AuditLogPage />}</SingleTenantGate>
    </section>
  );
}
