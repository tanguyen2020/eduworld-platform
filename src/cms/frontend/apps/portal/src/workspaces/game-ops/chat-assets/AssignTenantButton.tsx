// "Gán tenant" of a sticker pack (F082-AC-04, ADR-0029): only with platform.tenant_data.assign, it opens the shared
// assignment dialog of F057 (assignToTenants, module identity → module chat). Display only: the server authorizes.
import { Button } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { usePermissions } from '../../../permissions/usePermissions';
import { AssignToTenantsDialog } from '../../platform-admin/assignments/AssignToTenantsDialog';
import { registerPlatformI18n } from '../../platform-admin/i18n';
import type { ResourceType } from '../../platform-admin/api/common';

export const ASSIGN_PERMISSION = 'platform.tenant_data.assign';

export function AssignTenantButton({ resourceType, resourceId }: { resourceType: string; resourceId: string }) {
  const { t, i18n } = useTranslation();
  registerPlatformI18n(i18n); // the shared dialog speaks the platform.* keys of workspace D
  const { view } = usePermissions();
  const [open, setOpen] = useState(false);
  if (!view?.permissions.includes(ASSIGN_PERMISSION)) return null;
  return (
    <>
      <ActionGate actionKey="action.platform_admin.tenant_data.assign">
        <Button onClick={() => setOpen(true)}>{t('gameOps.chatAssets.assign')}</Button>
      </ActionGate>
      {/* sticker_pack joins the public ResourceType with the module chat platform-ops facade (Clarifications 2026-11-26) */}
      <AssignToTenantsDialog resourceType={resourceType as ResourceType} resourceId={resourceId} open={open} onClose={() => setOpen(false)} />
    </>
  );
}
