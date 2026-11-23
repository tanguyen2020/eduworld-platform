// Screen of `menu.school_lms.clubs` (F122 US9, F122-AC-23; FR-SOC-004, SOCIAL_SYSTEM §6.2): the clubs of one school
// (listClubs with `school_id` and `status`, cursor pages); approve a club waiting for approval, archive a club — both with a
// confirmation and Idempotency-Key. There is no screen of club chat content (staff never read club chat). Removing a member
// is a student-only route in module chat (F120: `removeClubMember` carries game_player) — open question of F122.
import { Button, Flex, Form, Modal, Tag } from 'antd';
import { SearchSelect } from '@eduworld/ops-ui';
import type { ColumnType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { SafetyError } from '../../../shared/chatSafety/ui';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { ResponsiveTable } from '../../../shell/ResponsiveTable';
import { SchoolSelect } from '../common/SchoolSelect';
import { SingleTenantGate } from '../common/scope';
import { PageTitle } from '../common/ui';
import { useClubActions, useClubs, type Club, type Schemas } from './api';

const STATUSES: Schemas['ClubStatus'][] = ['pending_approval', 'active', 'locked', 'archived'];

function ClubList() {
  const { t } = useTranslation();
  const [schoolId, setSchoolId] = useState<string | undefined>();
  const [status, setStatus] = useState<Schemas['ClubStatus'] | undefined>('pending_approval');
  const list = useClubs({ schoolId, status });
  const actions = useClubActions();
  const confirm = (kind: 'approve' | 'archive', club: Club) =>
    Modal.confirm({
      title: t(`chatSafety.clubs.${kind}Confirm`, { name: club.name }),
      okText: t(`chatSafety.clubs.${kind}`),
      cancelText: t('chatSafety.common.cancel'),
      okButtonProps: { danger: kind === 'archive' },
      onOk: () => actions[kind].mutateAsync(club.club_id),
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
