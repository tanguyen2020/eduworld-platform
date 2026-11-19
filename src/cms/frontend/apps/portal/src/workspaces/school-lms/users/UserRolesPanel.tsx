// UserRole of one staff member (F052 T035, F052-AC-16, F052-AC-17): list, grant, revoke (reason + confirmation,
// revokeTenantUserRole) with the notice that the change applies within `principal_cache_ttl` (runtime config, F051).
import { App, Button, Drawer, List, Space, Tag, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { usePortalRuntime } from '../../../app/runtime';
import { ActionGate } from '../../../permissions/ActionGate';
import { QueryError, writeErrorKey } from '../common/errors';
import { FIELDS } from '../common/fieldSchemas';
import { formatDate } from '../common/format';
import { ReasonModal } from '../common/ReasonModal';
import type { GrantMode, Role } from './canGrant';
import { GrantUserRoleDialog } from './GrantUserRoleDialog';
import { useRevokeUserRole, useUserRoleList, type UserRole } from './useUserRoles';
import type { User } from './useUsers';

export function UserRolesPanel({ user, roles, mode, onClose }: { user: User; roles: Role[]; mode: GrantMode; onClose: () => void }) {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const { config } = usePortalRuntime();
  const list = useUserRoleList(user.user_id, mode);
  const revoke = useRevokeUserRole();
  const [granting, setGranting] = useState(false);
  const [revoking, setRevoking] = useState<UserRole | null>(null);
  const effective = t('portal.school_lms.userRoles.effectiveWithin', { seconds: config.principalCacheTtlSeconds });
  return (
    <Drawer open title={t('portal.school_lms.userRoles.title', { name: user.display_name ?? user.username })} onClose={onClose} width="min(640px, 100vw)">
      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        <ActionGate actionKey="action.school_lms.access_control.assign_roles">
          <Button type="primary" onClick={() => setGranting(true)}>
            {t('portal.school_lms.userRoles.grant')}
          </Button>
        </ActionGate>
        {list.error ? <QueryError error={list.error} /> : null}
        <List<UserRole>
          loading={list.isFetching}
          dataSource={list.items}
          locale={{ emptyText: t('portal.school_lms.userRoles.none') }}
          renderItem={(r) => (
            <List.Item
              actions={[
                <ActionGate key="revoke" actionKey="action.school_lms.access_control.assign_roles">
                  <Button size="small" danger onClick={() => setRevoking(r)}>
                    {t('portal.school_lms.userRoles.revoke')}
                  </Button>
                </ActionGate>,
              ]}
            >
              <List.Item.Meta
                title={
                  <Space wrap>
                    <span>{r.role_code}</span>
                    <Tag>{t(`portal.school_lms.userRoles.scopes.${r.scope_type}`)}</Tag>
                    {r.source === 'sis' ? <Tag>{t('portal.school_lms.common.sisTag')}</Tag> : null}
                  </Space>
                }
                description={
                  <Typography.Text type="secondary">
                    {t('portal.school_lms.userRoles.validRange', { from: formatDate(r.valid_from), to: r.valid_to ? formatDate(r.valid_to) : t('portal.school_lms.userRoles.noEnd') })}
                  </Typography.Text>
                }
              />
            </List.Item>
          )}
        />
      </Space>
      {granting ? <GrantUserRoleDialog userId={user.user_id} roles={roles} mode={mode} onClose={() => setGranting(false)} /> : null}
      <ReasonModal
        open={!!revoking}
        title={t('portal.school_lms.userRoles.revokeTitle', { role: revoking?.role_code ?? '' })}
        schema={FIELDS.revokeReason}
        okText={t('portal.school_lms.userRoles.revoke')}
        danger
        confirmLoading={revoke.isPending}
        onCancel={() => setRevoking(null)}
        onSubmit={async (reason) => {
          if (!revoking) return;
          try {
            await revoke.mutateAsync({ userRoleId: revoking.user_role_id, reason });
            message.success(`${t('portal.school_lms.userRoles.revoked')} ${effective}`);
            setRevoking(null);
          } catch (e) {
            message.error(t(writeErrorKey(e)));
          }
        }}
      >
        {effective}
      </ReasonModal>
    </Drawer>
  );
}
