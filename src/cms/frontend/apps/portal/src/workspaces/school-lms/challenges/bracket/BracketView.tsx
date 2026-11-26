// Bracket of a school tournament for the organiser (F108 T028; F108-AC-15): nodes by bracket side and round, each with
// both entries (real names of the own school's students — portal staff view), status, winner, schedule and the
// organiser actions "Xác nhận kết quả" (confirmBracketNode) and "Điều chỉnh kết quả" (overrideBracketNode through
// OverrideDialog: mandatory reason + second confirmation); group standings when the format has groups. Which node may
// be confirmed or overridden is decided by the server (422 otherwise); the buttons only follow the node status.
// Presentational only (AGENTS R1).
import { Button, Card, Flex, Space, Table, Tag, Typography } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../../permissions/ActionGate';
import { formatDateTime } from '../../common/format';
import type { Bracket, BracketNode } from '../api/challenges';
import { EnumTag } from '../ChallengeUi';

type Standing = Bracket['groups'][number]['standings'][number];

export interface BracketViewProps {
  bracket: Bracket;
  /** Display name of an entry (real names of its students). */
  nameOf: (entryId: string | null | undefined) => string | undefined;
  onConfirm: (node: BracketNode) => void;
  onOverride: (node: BracketNode) => void;
  /** Node whose confirmation is running. */
  confirming?: string;
}

const SIDES: BracketNode['bracket'][] = ['winners', 'losers', 'grand_final', 'group'];
const CONFIRMABLE: BracketNode['status'][] = ['awaiting_confirm'];
const FINAL: BracketNode['status'][] = ['void', 'bye', 'pending'];

function Side({ node, entryId, nameOf }: { node: BracketNode; entryId: string | null | undefined; nameOf: BracketViewProps['nameOf'] }) {
  const { t } = useTranslation();
  if (!entryId) return <Typography.Text type="secondary">{node.status === 'bye' ? t('lms.challenges.bracket.bye') : t('lms.challenges.bracket.tbd')}</Typography.Text>;
  const winner = !!node.winner_entry_id && node.winner_entry_id === entryId;
  return (
    <Flex gap="small" align="center" wrap>
      <Typography.Text strong={winner}>{nameOf(entryId) ?? t('lms.challenges.entries.unknownStudent')}</Typography.Text>
      {winner ? <Tag>{t('lms.challenges.bracket.winner')}</Tag> : null}
    </Flex>
  );
}

function NodeCard({ node, nameOf, onConfirm, onOverride, confirming }: { node: BracketNode } & Omit<BracketViewProps, 'bracket'>) {
  const { t } = useTranslation();
  const confirmable = CONFIRMABLE.includes(node.status) || !!node.decision_required;
  const overridable = !!node.entry_a_id && !!node.entry_b_id && !FINAL.includes(node.status);
  return (
    <Card size="small" title={t('lms.challenges.bracket.match', { round: node.round, position: node.position })} data-testid={`node-${node.node_id}`} extra={<EnumTag group="nodeStatus" value={node.status} />}>
      <Flex vertical gap="small">
        <Side node={node} entryId={node.entry_a_id} nameOf={nameOf} />
        <Side node={node} entryId={node.entry_b_id} nameOf={nameOf} />
        {node.scheduled_at ? <Typography.Text type="secondary">{formatDateTime(node.scheduled_at)}</Typography.Text> : null}
        {node.confirm_after ? <Typography.Text type="secondary">{t('lms.challenges.bracket.confirmAfter', { at: formatDateTime(node.confirm_after) })}</Typography.Text> : null}
        {node.decision_required ? <Tag color="warning">{t('lms.challenges.bracket.decision')}</Tag> : null}
        {confirmable || overridable ? (
          <ActionGate actionKey="action.school_lms.challenges.manage">
            <Space wrap>
              {confirmable ? (
                <Button size="small" type="primary" loading={confirming === node.node_id} onClick={() => onConfirm(node)}>
                  {t('lms.challenges.bracket.confirm')}
                </Button>
              ) : null}
              {overridable ? (
                <Button size="small" onClick={() => onOverride(node)}>
                  {t('lms.challenges.bracket.override')}
                </Button>
              ) : null}
            </Space>
          </ActionGate>
        ) : null}
      </Flex>
    </Card>
  );
}

function Standings({ group, nameOf }: { group: Bracket['groups'][number]; nameOf: BracketViewProps['nameOf'] }) {
  const { t } = useTranslation();
  const columns: ColumnType<Standing>[] = [
    { key: 'rank', title: t('lms.challenges.bracket.standings.rank'), render: (_, s) => s.rank ?? '' },
    { key: 'entry', title: t('lms.challenges.bracket.standings.entry'), render: (_, s) => nameOf(s.entry_id) ?? t('lms.challenges.entries.unknownStudent') },
    { key: 'wins', title: t('lms.challenges.bracket.standings.wins'), dataIndex: 'wins' },
    { key: 'draws', title: t('lms.challenges.bracket.standings.draws'), dataIndex: 'draws' },
    { key: 'losses', title: t('lms.challenges.bracket.standings.losses'), dataIndex: 'losses' },
    { key: 'points', title: t('lms.challenges.bracket.standings.points'), dataIndex: 'points' },
  ];
  return <Table<Standing> rowKey="entry_id" size="small" pagination={false} columns={columns} dataSource={group.standings} title={() => t('lms.challenges.bracket.group', { code: group.group_code })} />;
}

export function BracketView({ bracket, ...rest }: BracketViewProps) {
  const { t } = useTranslation();
  const sides = SIDES.map((side) => {
    const nodes = bracket.nodes.filter((n) => n.bracket === side);
    const rounds = [...new Set(nodes.map((n) => n.round))].sort((a, b) => a - b);
    return { side, rounds: rounds.map((r) => ({ round: r, nodes: nodes.filter((n) => n.round === r).sort((a, b) => a.position - b.position) })) };
  }).filter((s) => s.rounds.length);
  if (!bracket.nodes.length && !bracket.groups.length) return <Typography.Text type="secondary">{t('lms.challenges.tournament.noBracket')}</Typography.Text>;
  return (
    <Flex vertical gap="large" data-testid="bracket-view">
      {bracket.groups.map((g) => (
        <Standings key={g.group_code} group={g} nameOf={rest.nameOf} />
      ))}
      {sides.map((s) => (
        <Flex vertical gap="small" key={s.side}>
          <Typography.Title level={4}>{t(`lms.challenges.bracketSide.${s.side}`)}</Typography.Title>
          <Flex gap="middle" wrap align="flex-start">
            {s.rounds.map((r) => (
              <Flex vertical gap="small" key={r.round}>
                {r.nodes.map((n) => (
                  <NodeCard key={n.node_id} node={n} {...rest} />
                ))}
              </Flex>
            ))}
          </Flex>
        </Flex>
      ))}
    </Flex>
  );
}
