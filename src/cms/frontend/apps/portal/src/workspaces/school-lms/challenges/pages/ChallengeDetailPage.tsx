// Detail of a grade challenge / school tournament (F108 T028; F108-AC-15): summary of the challenge (status, audience =
// grade ± students of one school with the server-computed number of effective students), and for a tournament its
// status, registrations with check-in, bracket and results — students shown with their real names (portal staff of the
// own school; the player view keeps nicknames, R16). Organiser actions, each calling exactly its F107 operation inside
// ActionGate(action.school_lms.challenges.manage): publishTournament, registerTournamentEntry (empty body = the
// audience), confirmTournamentEntries, generateTournamentBracket, withdrawTournamentEntry, confirmBracketNode,
// overrideBracketNode (reason + 2-step), cancelTournament / cancelChallenge (reason). The buttons follow the status
// shown; the server decides whether an action is allowed (AGENTS R1).
import { Button, Descriptions, Flex, Popconfirm, Space, Spin, Table, Tabs, Typography } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useParams } from 'react-router-dom';
import { ActionGate } from '../../../../permissions/ActionGate';
import { ApiError } from '../../../../shared/api/middleware/problem';
import { NotFoundResult, isHiddenResource } from '../../common/errors';
import { formatDateTime } from '../../common/format';
import { ReasonModal } from '../../common/ReasonModal';
import { useSchoolGrades } from '../../lms/api/targets';
import {
  useCancelChallenge,
  useChallenge,
  useConfirmEntries,
  useConfirmNode,
  useOverrideNode,
  useRegisterAudience,
  useSchoolStudentNames,
  useTournament,
  useTournamentAction,
  useTournamentBracket,
  useWithdrawEntry,
  type Bracket,
  type BracketNode,
  type Tournament,
  type TournamentEntry,
  type TournamentEntryResult,
} from '../api/challenges';
import { BracketView } from '../bracket/BracketView';
import { OverrideDialog } from '../bracket/OverrideDialog';
import { ChallengeProblem, EnumTag } from '../ChallengeUi';
import { CHALLENGE_FIELDS, gradeOf, includedOf, type Challenge } from '../model/challengeForm';

const OPEN: Tournament['status'][] = ['published', 'registration_open', 'registration_closed', 'seeding'];
const ENDED: Tournament['status'][] = ['completed', 'rewarded', 'archived', 'cancelled'];
const ACTIVE_ENTRY: TournamentEntry['status'][] = ['registered', 'confirmed', 'checked_in'];
const EMPTY_BRACKET: Bracket = { entries: [], format: 'single_elim', groups: [], nodes: [], status: 'draft', tournament_id: '' };

type NameOf = (entryId: string | null | undefined) => string | undefined;

function EntryTables({ entries, names, tournamentId, canWithdraw }: { entries: TournamentEntry[]; names: Map<string, string>; tournamentId: string; canWithdraw: boolean }) {
  const { t } = useTranslation();
  const withdraw = useWithdrawEntry(tournamentId);
  const realName = (e: TournamentEntry) => e.players.map((p) => names.get(p.student_id) ?? t('lms.challenges.entries.unknownStudent')).join(', ');
  const nickname = (e: TournamentEntry) => e.players.map((p) => p.nickname ?? '').filter(Boolean).join(', ');
  const columns: ColumnType<TournamentEntry>[] = [
    { key: 'student', title: t('lms.challenges.entries.student'), render: (_, e) => realName(e) },
    { key: 'nickname', title: t('lms.challenges.entries.nickname'), render: (_, e) => <Typography.Text type="secondary">{nickname(e)}</Typography.Text> },
    { key: 'status', title: t('lms.challenges.entries.status'), render: (_, e) => <EnumTag group="entryStatus" value={e.status} /> },
    { key: 'checkedIn', title: t('lms.challenges.entries.checkedIn'), render: (_, e) => formatDateTime(e.checked_in_at) },
    { key: 'seed', title: t('lms.challenges.entries.seed'), render: (_, e) => e.seed ?? '' },
    { key: 'group', title: t('lms.challenges.entries.group'), render: (_, e) => e.group_code ?? '' },
    {
      key: 'actions',
      title: '',
      render: (_, e) =>
        canWithdraw && ACTIVE_ENTRY.includes(e.status) ? (
          <ActionGate actionKey="action.school_lms.challenges.manage">
            <Popconfirm title={t('lms.challenges.entries.withdrawConfirm')} okText={t('lms.challenges.entries.yes')} cancelText={t('lms.challenges.entries.no')} onConfirm={() => withdraw.mutate(e.entry_id)}>
              <Button size="small" danger loading={withdraw.isPending && withdraw.variables === e.entry_id}>
                {t('lms.challenges.entries.withdraw')}
              </Button>
            </Popconfirm>
          </ActionGate>
        ) : null,
    },
  ];
  return (
    <Flex vertical gap="small">
      <ChallengeProblem error={withdraw.error} testId="withdraw-error" />
      <div data-testid="entry-table">
        <Table<TournamentEntry> rowKey="entry_id" size="small" pagination={false} columns={columns} dataSource={entries} locale={{ emptyText: t('lms.challenges.entries.empty') }} scroll={{ x: true }} />
      </div>
    </Flex>
  );
}

function ResultTable({ entries, names }: { entries: TournamentEntry[]; names: Map<string, string> }) {
  const { t } = useTranslation();
  const ranked = entries.filter((e) => e.final_rank != null).sort((a, b) => (a.final_rank ?? 0) - (b.final_rank ?? 0));
  const columns: ColumnType<TournamentEntry>[] = [
    { key: 'rank', title: t('lms.challenges.entries.rank'), render: (_, e) => e.final_rank },
    { key: 'student', title: t('lms.challenges.entries.student'), render: (_, e) => e.players.map((p) => names.get(p.student_id) ?? t('lms.challenges.entries.unknownStudent')).join(', ') },
    { key: 'status', title: t('lms.challenges.entries.status'), render: (_, e) => <EnumTag group="entryStatus" value={e.status} /> },
  ];
  return (
    <div data-testid="result-table">
      <Table<TournamentEntry> rowKey="entry_id" size="small" pagination={false} columns={columns} dataSource={ranked} locale={{ emptyText: t('lms.challenges.entries.noResult') }} />
    </div>
  );
}

function TournamentPanel({ id, names }: { id: string; names: Map<string, string> }) {
  const { t } = useTranslation();
  const q = useTournament(id);
  const b = useTournamentBracket(id);
  const publish = useTournamentAction(id, 'publish');
  const generate = useTournamentAction(id, 'generate-bracket');
  const cancel = useTournamentAction(id, 'cancel');
  const register = useRegisterAudience(id);
  const confirmEntries = useConfirmEntries(id);
  const confirmNode = useConfirmNode(id);
  const override = useOverrideNode(id);
  const [cancelOpen, setCancelOpen] = useState(false);
  const [overriding, setOverriding] = useState<BracketNode | null>(null);
  const [registered, setRegistered] = useState<TournamentEntryResult | null>(null);
  if (q.error) return <ChallengeProblem error={q.error} />;
  const tour = q.data;
  if (!tour) return <Spin />;
  // no bracket yet (404 before generation) = no entry list either
  const missing = b.error instanceof ApiError && b.error.status === 404;
  const bracket = b.data ?? EMPTY_BRACKET;
  const entryById = new Map(bracket.entries.map((e) => [e.entry_id, e]));
  const nameOf: NameOf = (entryId) => {
    const e = entryId ? entryById.get(entryId) : undefined;
    const known = e?.players.map((p) => names.get(p.student_id)).filter((n): n is string => !!n) ?? [];
    return known.length ? known.join(', ') : undefined;
  };
  const open = OPEN.includes(tour.status);
  const ended = ENDED.includes(tour.status);
  const actionError = publish.error ?? register.error ?? confirmEntries.error ?? generate.error ?? cancel.error ?? confirmNode.error;
  return (
    <Flex vertical gap="middle" data-testid="tournament-panel">
      <Typography.Title level={3}>{t('lms.challenges.tournament.heading')}</Typography.Title>
      <Descriptions bordered size="small" column={{ xs: 1, md: 2 }}>
        <Descriptions.Item label={t('lms.challenges.tournament.status')}>
          <EnumTag group="tournamentStatus" value={tour.status} testId="tournament-status" />
        </Descriptions.Item>
        <Descriptions.Item label={t('lms.challenges.tournament.format')}>
          <EnumTag group="tournamentFormat" value={tour.format} />
          <EnumTag group="participantType" value={tour.participant_type} />
        </Descriptions.Item>
        <Descriptions.Item label={t('lms.challenges.tournament.seeding')}>{tour.seeding_policy ? t(`lms.challenges.seedingPolicy.${tour.seeding_policy}`) : ''}</Descriptions.Item>
        <Descriptions.Item label={t('lms.challenges.tournament.entries')}>{tour.entry_count ?? bracket.entries.length}</Descriptions.Item>
        <Descriptions.Item label={t('lms.challenges.tournament.registrationOpen')}>{formatDateTime(tour.registration_open_at)}</Descriptions.Item>
        <Descriptions.Item label={t('lms.challenges.tournament.registrationClose')}>{formatDateTime(tour.registration_closes_at)}</Descriptions.Item>
        <Descriptions.Item label={t('lms.challenges.tournament.checkIn')}>{formatDateTime(tour.check_in_opens_at)}</Descriptions.Item>
        <Descriptions.Item label={t('lms.challenges.tournament.reward')}>{tour.reward_summary ?? ''}</Descriptions.Item>
      </Descriptions>
      <ActionGate actionKey="action.school_lms.challenges.manage">
        <Space wrap>
          {tour.status === 'draft' ? (
            <Button type="primary" loading={publish.isPending} onClick={() => publish.mutate({})}>
              {t('lms.challenges.tournament.publish')}
            </Button>
          ) : null}
          {open ? (
            <>
              <Button loading={register.isPending} onClick={() => register.mutate({}, { onSuccess: setRegistered })}>
                {t('lms.challenges.tournament.register')}
              </Button>
              <Button loading={confirmEntries.isPending} onClick={() => confirmEntries.mutate({})}>
                {t('lms.challenges.tournament.confirmEntries')}
              </Button>
              <Button type="primary" loading={generate.isPending} onClick={() => generate.mutate({})}>
                {t('lms.challenges.tournament.generate')}
              </Button>
            </>
          ) : null}
          {!ended ? (
            <Button danger onClick={() => setCancelOpen(true)}>
              {t('lms.challenges.tournament.cancel')}
            </Button>
          ) : null}
        </Space>
      </ActionGate>
      {registered ? (
        <Typography.Text role="status" data-testid="register-result">
          {t('lms.challenges.tournament.registered', { n: registered.entries.length, m: registered.skipped?.length ?? 0 })}
        </Typography.Text>
      ) : null}
      <ChallengeProblem error={actionError} testId="action-error" />
      {b.error && !missing ? <ChallengeProblem error={b.error} /> : null}
      <Tabs
        items={[
          { key: 'entries', label: t('lms.challenges.tournament.tabs.entries'), children: <EntryTables entries={bracket.entries} names={names} tournamentId={id} canWithdraw={!ended} /> },
          {
            key: 'bracket',
            label: t('lms.challenges.tournament.tabs.bracket'),
            children: <BracketView bracket={bracket} nameOf={nameOf} onConfirm={(n) => confirmNode.mutate(n.node_id)} confirming={confirmNode.isPending ? confirmNode.variables : undefined} onOverride={setOverriding} />,
          },
          { key: 'results', label: t('lms.challenges.tournament.tabs.results'), children: <ResultTable entries={bracket.entries} names={names} /> },
        ]}
      />
      <ReasonModal
        open={cancelOpen}
        title={t('lms.challenges.tournament.cancelTitle')}
        okText={t('lms.challenges.tournament.cancel')}
        danger
        schema={CHALLENGE_FIELDS.cancelReason}
        confirmLoading={cancel.isPending}
        onCancel={() => setCancelOpen(false)}
        onSubmit={(reason) => {
          setCancelOpen(false);
          cancel.mutate({ reason });
        }}
      >
        {t('lms.challenges.tournament.cancelHelp')}
      </ReasonModal>
      {overriding ? (
        <OverrideDialog
          node={overriding}
          nameOf={nameOf}
          pending={override.isPending}
          error={override.error}
          onCancel={() => setOverriding(null)}
          onSubmit={(winner, reason) => override.mutate({ nodeId: overriding.node_id, winner_entry_id: winner, reason }, { onSuccess: () => setOverriding(null) })}
        />
      ) : null}
    </Flex>
  );
}

function ChallengeActions({ c }: { c: Challenge }) {
  const { t } = useTranslation();
  const cancel = useCancelChallenge(c.challenge_id);
  const [open, setOpen] = useState(false);
  // a tournament is cancelled with its own operation (cancelTournament) in the tournament panel
  if (c.tournament_id || (c.status !== 'scheduled' && c.status !== 'running')) return <ChallengeProblem error={cancel.error} />;
  return (
    <Flex vertical gap="small">
      <ActionGate actionKey="action.school_lms.challenges.manage">
        <Space wrap>
          <Button danger onClick={() => setOpen(true)}>
            {t('lms.challenges.detail.cancel')}
          </Button>
        </Space>
      </ActionGate>
      <ChallengeProblem error={cancel.error} testId="cancel-error" />
      <ReasonModal
        open={open}
        title={t('lms.challenges.detail.cancelTitle')}
        okText={t('lms.challenges.detail.cancel')}
        danger
        schema={CHALLENGE_FIELDS.cancelReason}
        confirmLoading={cancel.isPending}
        onCancel={() => setOpen(false)}
        onSubmit={(reason) => {
          setOpen(false);
          cancel.mutate({ reason });
        }}
      >
        {t('lms.challenges.detail.cancelHelp')}
      </ReasonModal>
    </Flex>
  );
}

export function ChallengeDetailPage() {
  const { t } = useTranslation();
  const { challengeId } = useParams();
  const q = useChallenge(challengeId);
  const c = q.data;
  const grade = gradeOf(c);
  const included = includedOf(c);
  const school = useSchoolGrades(c?.school_id);
  // real names: the grade of the challenge, plus the other grades of the school when students were added
  const names = useSchoolStudentNames(c?.tournament_id ? c.school_id : undefined, grade ? [grade, ...(included.length ? school.grades.filter((g) => g !== grade) : [])] : []);
  if (q.error) return isHiddenResource(q.error) ? <NotFoundResult /> : <ChallengeProblem error={q.error} />;
  if (!c) return <Spin />;
  return (
    <Flex vertical gap="middle" data-testid="challenge-detail">
      <Link to="..">{t('portal.school_lms.common.back')}</Link>
      <Typography.Title level={2}>{c.name}</Typography.Title>
      <Descriptions bordered size="small" column={{ xs: 1, md: 2 }}>
        <Descriptions.Item label={t('lms.challenges.detail.status')}>
          <EnumTag group="status" value={c.status} testId="challenge-status" />
        </Descriptions.Item>
        <Descriptions.Item label={t('lms.challenges.detail.kind')}>
          <EnumTag group="kind" value={c.kind} />
          <EnumTag group="format" value={c.format} />
        </Descriptions.Item>
        <Descriptions.Item label={t('lms.challenges.detail.grade')}>{grade ? t('lms.challenges.gradeN', { n: grade }) : ''}</Descriptions.Item>
        <Descriptions.Item label={t('lms.challenges.detail.audience')}>
          <span data-testid="challenge-audience">{c.audience_count ?? ''}</span>
        </Descriptions.Item>
        <Descriptions.Item label={t('lms.challenges.detail.include')}>{included.length}</Descriptions.Item>
        <Descriptions.Item label={t('lms.challenges.detail.exclude')}>{c.exclude_student_ids?.length ?? 0}</Descriptions.Item>
        <Descriptions.Item label={t('lms.challenges.detail.startsAt')}>{formatDateTime(c.starts_at)}</Descriptions.Item>
        <Descriptions.Item label={t('lms.challenges.detail.endsAt')}>{formatDateTime(c.ends_at)}</Descriptions.Item>
        {c.participant_count != null ? <Descriptions.Item label={t('lms.challenges.detail.participants')}>{c.participant_count}</Descriptions.Item> : null}
        {c.cancel_reason ? <Descriptions.Item label={t('lms.challenges.detail.cancelReason')}>{c.cancel_reason}</Descriptions.Item> : null}
      </Descriptions>
      <ChallengeActions c={c} />
      {c.tournament_id ? <TournamentPanel id={c.tournament_id} names={names} /> : null}
    </Flex>
  );
}
