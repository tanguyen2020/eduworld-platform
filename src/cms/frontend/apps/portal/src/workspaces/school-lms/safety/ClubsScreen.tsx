// Screen of `menu.school_lms.clubs` (F122 US9, F122-AC-23; FR-SOC-004, SOCIAL_SYSTEM §6.2): the clubs of one school
// (listClubs with `school_id` and `status`, cursor pages); approve a club waiting for approval, archive a club — both with a
// confirmation and Idempotency-Key. "Thành viên" lists the members of a club (listSchoolClubMembers, nicknames only — R16,
// cursor pages) and "Gỡ thành viên" removes one with a mandatory reason (removeSchoolClubMember: staff route of F122 T061,
// social.club.approve in the school of the club; never the owner). There is no screen of club chat content (staff never
// read club chat).
import { Button, Flex, Form, Input, Modal, Table, Tag, Typography } from 'antd';
import { openapiInputProps, openapiRules, OPS_UI_NS, SearchSelect } from '@eduworld/ops-ui';
import type { ColumnType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { LIMITS, SafetyError } from '../../../shared/chatSafety/ui';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { ResponsiveTable } from '../../../shell/ResponsiveTable';
import { SchoolSelect } from '../common/SchoolSelect';
import { SingleTenantGate } from '../common/scope';
import { PageTitle } from '../common/ui';
import { useClubActions, useClubMembers, useClubs, useRemoveClubMember, type Club, type ClubMember, type Schemas } from './api';

const STATUSES: Schemas['ClubStatus'][] = ['pending_approval', 'active', 'locked', 'archived'];

/**
 * Confirmation of the removal of a member, inside the members dialog (one modal on screen): reason required (1–500,
 * counter), errors by `code`, Idempotency-Key kept for a retry of the same intent.
 */
function RemoveMemberForm({ club, member, onClose }: { club: Club; member: ClubMember; onClose: () => void }) {
  const { t } = useTranslation();
  const ops = useTranslation(OPS_UI_NS);
  const [form] = Form.useForm<{ reason: string }>();
  const remove = useRemoveClubMember();
  const submit = (v: { reason: string }) => remove.mutate({ clubId: club.club_id, userId: member.user_id, reason: v.reason.trim() }, { onSuccess: onClose });
  const title = t('chatSafety.clubs.removeTitle', { name: member.nickname });
  return (
    <section role="group" aria-label={title}>
      <Typography.Title level={3}>{title}</Typography.Title>
      <Form form={form} layout="vertical" name="club_member_remove" onFinish={submit}>
        <Form.Item
          name="reason"
          label={t('chatSafety.clubs.removeReason')}
          rules={[{ required: true, whitespace: true, message: t('chatSafety.common.reasonRequired') }, ...openapiRules(LIMITS.clubRemoveReason, { t: ops.t })]}
        >
          <Input.TextArea autoSize={{ minRows: 2, maxRows: 5 }} {...openapiInputProps(LIMITS.clubRemoveReason)} />
        </Form.Item>
        {remove.error ? <ProblemMessage error={remove.error} /> : null}
        <Flex gap="small">
          <Button type="primary" danger htmlType="submit" loading={remove.isPending}>
            {t('chatSafety.clubs.remove')}
          </Button>
          <Button onClick={onClose}>{t('chatSafety.common.cancel')}</Button>
        </Flex>
      </Form>
    </section>
  );
}

/** Members of one club (cursor pages); "Gỡ thành viên" behind the approve action of the catalogue. */
function ClubMembersDialog({ club, onClose }: { club: Club; onClose: () => void }) {
  const { t } = useTranslation();
  const list = useClubMembers(club.club_id);
  const [removing, setRemoving] = useState<ClubMember | null>(null);
  const columns: ColumnType<ClubMember>[] = [
    { key: 'nickname', title: t('chatSafety.clubs.nickname'), render: (_, r) => r.nickname },
    { key: 'role', title: t('chatSafety.clubs.role'), render: (_, r) => <Tag>{t(`chatSafety.clubRole.${r.role}`)}</Tag> },
    {
      key: 'actions',
      title: t('chatSafety.common.actions'),
      render: (_, r) =>
        r.role === 'owner' ? null : (
          <ActionGate actionKey="action.school_lms.clubs.approve">
            <Button size="small" danger onClick={() => setRemoving(r)}>
              {t('chatSafety.clubs.remove')}
            </Button>
          </ActionGate>
        ),
    },
  ];
  return (
    <Modal open title={t('chatSafety.clubs.membersOf', { name: club.name })} onCancel={onClose} footer={null} destroyOnHidden width="40rem">
      <Flex vertical gap="middle">
        {list.error ? <SafetyError error={list.error} /> : null}
        <Table rowKey="user_id" columns={columns} dataSource={list.rows} loading={list.isFetching} pagination={false} locale={{ emptyText: t('chatSafety.common.empty') }} />
        {list.hasNextPage ? (
          <Button onClick={() => list.fetchNextPage()} loading={list.isFetchingNextPage}>
            {t('chatSafety.common.loadMore')}
          </Button>
        ) : null}
      </Flex>
      {removing ? <RemoveMemberForm key={removing.user_id} club={club} member={removing} onClose={() => setRemoving(null)} /> : null}
    </Modal>
  );
}

function ClubList() {
  const { t } = useTranslation();
  const [schoolId, setSchoolId] = useState<string | undefined>();
  const [status, setStatus] = useState<Schemas['ClubStatus'] | undefined>('pending_approval');
  const list = useClubs({ schoolId, status });
  const actions = useClubActions();
  const [membersOf, setMembersOf] = useState<Club | null>(null);
  const confirm = (kind: 'approve' | 'archive', club: Club) =>
    Modal.confirm({
      title: t(`chatSafety.clubs.${kind}Confirm`, { name: club.name }),
      okText: t(`chatSafety.clubs.${kind}`),
      cancelText: t('chatSafety.common.cancel'),
      okButtonProps: { danger: kind === 'archive' },
      // the error is shown under the list (ProblemMessage); the dialog closes without an unhandled rejection
      onOk: () => actions[kind].mutateAsync(club.club_id).catch(() => undefined),
    });
  const columns: ColumnType<Club>[] = [
    { key: 'title', title: t('chatSafety.clubs.name'), render: (_, r) => r.name },
    { key: 'type', title: t('chatSafety.clubs.type'), render: (_, r) => t(`chatSafety.clubType.${r.club_type}`) },
    { key: 'members', title: t('chatSafety.clubs.members'), render: (_, r) => `${r.member_count}/${r.max_members}` },
    { key: 'status', title: t('chatSafety.clubs.status'), render: (_, r) => <Tag>{t(`chatSafety.clubStatus.${r.status}`)}</Tag> },
    {
      key: 'actions',
      title: t('chatSafety.common.actions'),
      render: (_, r) => (
        <Flex gap="small" wrap>
          {r.status !== 'pending_approval' ? (
            <Button size="small" onClick={() => setMembersOf(r)}>
              {t('chatSafety.clubs.members')}
            </Button>
          ) : null}
          <ActionGate actionKey="action.school_lms.clubs.approve">
          <Flex gap="small" wrap>
            {r.status === 'pending_approval' ? (
              <Button size="small" type="primary" onClick={() => confirm('approve', r)}>
                {t('chatSafety.clubs.approve')}
              </Button>
            ) : null}
            {r.status !== 'archived' ? (
              <Button size="small" danger onClick={() => confirm('archive', r)}>
                {t('chatSafety.clubs.archive')}
              </Button>
            ) : null}
          </Flex>
          </ActionGate>
        </Flex>
      ),
    },
  ];
  return (
    <Flex vertical gap="middle">
      <Form layout="inline" name="club_filters">
        <Form.Item label={t('chatSafety.common.school')}>
          <SchoolSelect value={schoolId} onChange={setSchoolId} />
        </Form.Item>
        <Form.Item label={t('chatSafety.clubs.status')}>
          <SearchSelect
            aria-label={t('chatSafety.clubs.status')}
            allowClear
            value={status}
            onChange={setStatus}
            options={STATUSES.map((s) => ({ value: s, label: t(`chatSafety.clubStatus.${s}`) }))}
            style={{ minWidth: '10rem' }}
          />
        </Form.Item>
      </Form>
      {list.error ? <SafetyError error={list.error} /> : null}
      {actions.approve.error ? <ProblemMessage error={actions.approve.error} /> : null}
      {actions.archive.error ? <ProblemMessage error={actions.archive.error} /> : null}
      <ResponsiveTable<Club>
        rowKey="club_id"
        columns={columns}
        data={list.rows}
        loading={list.isFetching}
        cardTitle={(r) => r.name}
        hasMore={list.hasNextPage}
        onLoadMore={() => void list.fetchNextPage()}
        loadMoreLabel={t('chatSafety.common.loadMore')}
      />
      {membersOf ? <ClubMembersDialog club={membersOf} onClose={() => setMembersOf(null)} /> : null}
    </Flex>
  );
}

export function ClubsScreen({ title }: { title: string }) {
  return (
    <section>
      <PageTitle title={title} />
      <SingleTenantGate>{() => <ClubList />}</SingleTenantGate>
    </section>
  );
}
