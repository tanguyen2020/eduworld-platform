// Screen of `menu.platform_admin.moderation_switches` (F122 US4; F122-AC-10, AC-11, AC-12, AC-27; ADR-0030, ADR-0078):
// one row for the platform and one per tenant of the topbar filter — `enabled_all` and the 4 switchable steps; PII and
// grooming always run ("Luôn bật", no switch); a step turned off by the platform is locked on the tenant rows with a note.
// Turning anything off opens a confirmation stating the risk for students aged 6–15, with a mandatory reason (≥ 10, counter)
// and a mandatory `expires_at` within `moderation_switch_max_duration`; the change becomes a request "Chờ phê duyệt" that
// another System Admin approves (the proposer never sees the Approve button). Errors are shown by `code`;
// `MODERATION_SWITCH_REASON_REQUIRED` marks the reason field. History tab: before/after, reason, expiry, actor, source.
import { Alert, Button, Flex, Form, Input, Modal, Switch, Table, Tabs, Tag, Typography, theme } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { AlwaysOnLock, openapiInputProps, openapiRules, OPS_UI_NS, SearchSelect } from '@eduworld/ops-ui';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { useMe } from '../../../shared/api/hooks/useMe';
import { ApiError } from '../../../shared/api/middleware/problem';
import { LIMITS, SafetyError } from '../../../shared/chatSafety/ui';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { TenantFilter } from '../../../shell/TenantControls';
import { useMediaQuery } from '../../../shell/useMediaQuery';
import { formatTime } from '../ui';
import { useApproveSwitch, useSetSwitches, useSwitches, useSwitchHistory, type Steps, type SwitchChange, type SwitchRequest, type SwitchState } from './api';

export const STEPS: (keyof Steps)[] = ['sensitive_terms', 'spam', 'bullying', 'async_analysis'];
/** moderation_switch_max_duration (CONFIG_DEFAULTS, ADR-0078): the server refuses a longer expiry anyway. */
export const SWITCH_MAX_HOURS = 24;
const DURATIONS = [1, 2, 4, 8, 12, SWITCH_MAX_HOURS];

type Target = 'enabled_all' | keyof Steps;

interface Change {
  level: SwitchState;
  target: Target;
  on: boolean;
}

/** Full configuration of the level after the change (the API takes the whole level). */
export function nextConfig(level: SwitchState, target: Target, on: boolean): { enabled_all: boolean; steps: Steps } {
  const base = level.configured ?? level.effective;
  const steps = { ...base.steps };
  let enabledAll = base.enabled_all;
  if (target === 'enabled_all') enabledAll = on;
  else steps[target] = on;
  return { enabled_all: enabledAll, steps };
}

function SwitchConfirmModal({ change, onClose }: { change: Change; onClose: () => void }) {
  const { t } = useTranslation();
  const ops = useTranslation(OPS_UI_NS);
  const [form] = Form.useForm<{ reason: string; hours?: number }>();
  const set = useSetSwitches();
  const [queued, setQueued] = useState(false);
  const disabling = !change.on;
  const label = change.target === 'enabled_all' ? t('chatSafety.switches.all') : t(`chatSafety.step.${change.target}`);
  const submit = (v: { reason: string; hours?: number }) => {
    const cfg = nextConfig(change.level, change.target, change.on);
    const tenant = change.level.scope === 'tenant' && change.level.tenant_id ? { tenant_id: change.level.tenant_id } : {};
    set.mutate(
      {
        scope: change.level.scope,
        ...tenant,
        ...cfg,
        reason: v.reason.trim(),
        ...(disabling && v.hours ? { expires_at: new Date(Date.now() + v.hours * 3_600_000).toISOString() } : {}),
      },
      {
        onSuccess: (res) => {
          if ((res as SwitchRequest).status === 'pending_approval') setQueued(true);
          else onClose();
        },
        onError: (e) => {
          if (e instanceof ApiError && e.code === 'MODERATION_SWITCH_REASON_REQUIRED') form.setFields([{ name: 'reason', errors: [t('error.MODERATION_SWITCH_REASON_REQUIRED')] }]);
          if (e instanceof ApiError && e.code === 'MODERATION_SWITCH_EXPIRY_REQUIRED') form.setFields([{ name: 'hours', errors: [t('error.MODERATION_SWITCH_EXPIRY_REQUIRED')] }]);
        },
      },
    );
  };
  return (
    <Modal open title={disabling ? t('chatSafety.switches.disableTitle', { step: label }) : t('chatSafety.switches.enableTitle', { step: label })} onCancel={onClose} footer={null} destroyOnHidden>
      {queued ? (
        <Flex vertical gap="small">
          <Alert type="info" showIcon message={t('chatSafety.switches.pending')} description={t('chatSafety.switches.pendingHelp')} />
          <Button onClick={onClose}>{t('chatSafety.common.close')}</Button>
        </Flex>
      ) : (
        <Form form={form} layout="vertical" name="switch_change" onFinish={submit}>
          {disabling ? <Alert type="error" showIcon message={t('chatSafety.switches.risk')} /> : null}
          <Form.Item name="reason" label={t('chatSafety.common.reason')} rules={openapiRules(LIMITS.switchReason, { required: true, t: ops.t })}>
            <Input.TextArea autoSize={{ minRows: 2, maxRows: 6 }} {...openapiInputProps(LIMITS.switchReason)} />
          </Form.Item>
          {disabling ? (
            <Form.Item name="hours" label={t('chatSafety.switches.expires', { max: SWITCH_MAX_HOURS })} rules={[{ required: true, message: t('chatSafety.switches.expiresRequired') }]}>
              <SearchSelect aria-label={t('chatSafety.switches.expires', { max: SWITCH_MAX_HOURS })} options={DURATIONS.map((h) => ({ value: h, label: t('chatSafety.switches.hours', { count: h }) }))} />
            </Form.Item>
          ) : null}
          {set.error ? <ProblemMessage error={set.error} /> : null}
          <Button type="primary" danger={disabling} htmlType="submit" loading={set.isPending}>
            {disabling ? t('chatSafety.switches.requestDisable') : t('chatSafety.switches.enable')}
          </Button>
        </Form>
      )}
    </Modal>
  );
}

function PendingRequests({ levels, me }: { levels: SwitchState[]; me?: string }) {
  const { t } = useTranslation();
  const approve = useApproveSwitch();
  const requests = levels.flatMap((l) => (l.pending_requests ?? []).map((r) => ({ ...r, levelName: l.scope === 'platform' ? t('chatSafety.switches.platform') : (l.tenant_name ?? l.tenant_id ?? '') })));
  if (!requests.length) return null;
  const columns: ColumnsType<(typeof requests)[number]> = [
    { key: 'level', title: t('chatSafety.switches.level'), render: (_, r) => r.levelName },
    {
      key: 'steps',
      title: t('chatSafety.switches.off'),
      render: (_, r) => (
        <Flex wrap gap="small">
          {!r.enabled_all ? <Tag color="error">{t('chatSafety.switches.all')}</Tag> : null}
          {STEPS.filter((s) => !r.steps[s]).map((s) => (
            <Tag key={s} color="error">
              {t(`chatSafety.step.${s}`)}
            </Tag>
          ))}
        </Flex>
      ),
    },
    { key: 'reason', title: t('chatSafety.common.reason'), render: (_, r) => r.reason },
    { key: 'expires', title: t('chatSafety.switches.until'), render: (_, r) => formatTime(r.expires_at) },
    { key: 'status', title: t('chatSafety.switches.state'), render: () => <Tag color="warning">{t('chatSafety.switches.pending')}</Tag> },
    {
      key: 'actions',
      title: t('chatSafety.common.actions'),
      render: (_, r) =>
        r.proposed_by === me ? (
          <Typography.Text type="secondary">{t('chatSafety.switches.ownRequest')}</Typography.Text>
        ) : (
          <ActionGate actionKey="action.platform_admin.moderation_switches.toggle">
            <Button size="small" type="primary" loading={approve.isPending} onClick={() => approve.mutate({ request_id: r.request_id, tenant_id: r.tenant_id ?? undefined })}>
              {t('chatSafety.switches.approve')}
            </Button>
          </ActionGate>
        ),
    },
  ];
  return (
    <section aria-label={t('chatSafety.switches.pendingTitle')}>
      <Typography.Title level={2}>{t('chatSafety.switches.pendingTitle')}</Typography.Title>
      {approve.error ? <ProblemMessage error={approve.error} /> : null}
      <Table rowKey="request_id" columns={columns} dataSource={requests} pagination={false} scroll={{ x: true }} />
    </section>
  );
}

function SwitchTable({ large }: { large: boolean }) {
  const { t } = useTranslation();
  const q = useSwitches();
  const me = useMe();
  const [change, setChange] = useState<Change | null>(null);
  const platform = q.platform;
  const platformOff = (s: keyof Steps) => platform ? !platform.effective.enabled_all || !platform.effective.steps[s] : false;
  const rows = platform ? [platform, ...q.rows] : [];
  const cell = (level: SwitchState, target: Target) => {
    const on = target === 'enabled_all' ? level.effective.enabled_all : level.effective.steps[target];
    const lockedByPlatform = level.scope === 'tenant' && (target === 'enabled_all' ? !platform!.effective.enabled_all : platformOff(target));
    const name = target === 'enabled_all' ? t('chatSafety.switches.all') : t(`chatSafety.step.${target}`);
    if (lockedByPlatform) {
      return (
        <Flex align="center" gap="small" wrap>
          <Switch checked={false} disabled aria-label={t('chatSafety.switches.switchOf', { step: name, level: level.tenant_name ?? '' })} />
          <Typography.Text type="secondary">{t('chatSafety.switches.lockedByPlatform')}</Typography.Text>
        </Flex>
      );
    }
    return (
      <ActionGate actionKey="action.platform_admin.moderation_switches.toggle" mode="disable">
        <Switch
          checked={on}
          disabled={!large}
          aria-label={t('chatSafety.switches.switchOf', { step: name, level: level.scope === 'platform' ? t('chatSafety.switches.platform') : (level.tenant_name ?? '') })}
          onChange={(v) => setChange({ level, target, on: v })}
        />
      </ActionGate>
    );
  };
  const columns: ColumnsType<SwitchState> = [
    { key: 'level', title: t('chatSafety.switches.level'), render: (_, r) => (r.scope === 'platform' ? <Tag>{t('chatSafety.switches.platform')}</Tag> : (r.tenant_name ?? r.tenant_id)) },
    { key: 'all', title: t('chatSafety.switches.all'), render: (_, r) => cell(r, 'enabled_all') },
    ...STEPS.map((s) => ({ key: s, title: t(`chatSafety.step.${s}`), render: (_: unknown, r: SwitchState) => cell(r, s) })),
    { key: 'pii', title: t('chatSafety.step.pii'), render: () => <AlwaysOnLock /> },
    { key: 'grooming', title: t('chatSafety.step.grooming'), render: () => <AlwaysOnLock /> },
    { key: 'until', title: t('chatSafety.switches.until'), render: (_, r) => formatTime(r.configured?.expires_at ?? null) },
  ];
  return (
    <Flex vertical gap="middle">
      <TenantFilter />
      <Alert type="info" showIcon message={t('chatSafety.switches.alwaysOnNote')} />
      {q.error ? <SafetyError error={q.error} /> : null}
      <Table rowKey={(r) => r.tenant_id ?? 'platform'} columns={columns} dataSource={rows} loading={q.isFetching && !q.data} pagination={false} scroll={{ x: true }} />
      {q.hasNextPage ? (
        <Button onClick={() => q.fetchNextPage()} loading={q.isFetchingNextPage}>
          {t('chatSafety.common.loadMore')}
        </Button>
      ) : null}
      <PendingRequests levels={rows} me={me.data?.user_id} />
      {change ? <SwitchConfirmModal change={change} onClose={() => setChange(null)} /> : null}
    </Flex>
  );
}

function stepsOff(steps: Steps, all?: boolean): string[] {
  return [...(all === false ? ['enabled_all'] : []), ...STEPS.filter((s) => !steps[s])];
}

function History() {
  const { t } = useTranslation();
  const list = useSwitchHistory();
  const names = (off: string[]) => (off.length ? off.map((s) => (s === 'enabled_all' ? t('chatSafety.switches.all') : t(`chatSafety.step.${s}`))).join(', ') : t('chatSafety.switches.allOn'));
  const columns: ColumnsType<SwitchChange> = [
    { key: 'at', title: t('chatSafety.switches.changedAt'), render: (_, r) => formatTime(r.changed_at) },
    { key: 'level', title: t('chatSafety.switches.level'), render: (_, r) => (r.scope === 'platform' ? t('chatSafety.switches.platform') : (r.tenant_id ?? '')) },
    { key: 'before', title: t('chatSafety.switches.before'), render: (_, r) => names(stepsOff(r.before, r.enabled_all_before)) },
    { key: 'after', title: t('chatSafety.switches.after'), render: (_, r) => names(stepsOff(r.after, r.enabled_all_after)) },
    { key: 'reason', title: t('chatSafety.common.reason'), render: (_, r) => r.reason },
    { key: 'expires', title: t('chatSafety.switches.until'), render: (_, r) => formatTime(r.expires_at ?? null) },
    { key: 'actor', title: t('chatSafety.switches.actor'), render: (_, r) => (r.actor_id ? <Typography.Text code>{r.actor_id.slice(-8)}</Typography.Text> : t('chatSafety.switches.system')) },
    { key: 'approver', title: t('chatSafety.switches.approver'), render: (_, r) => (r.approved_by ? <Typography.Text code>{r.approved_by.slice(-8)}</Typography.Text> : '') },
    { key: 'source', title: t('chatSafety.switches.source'), render: (_, r) => <Tag>{t(`chatSafety.switchSource.${r.source ?? 'manual'}`)}</Tag> },
  ];
  return (
    <Flex vertical gap="middle">
      <TenantFilter />
      {list.error ? <SafetyError error={list.error} /> : null}
      <Table rowKey="change_id" columns={columns} dataSource={list.rows} loading={list.isFetching} pagination={false} scroll={{ x: true }} locale={{ emptyText: t('chatSafety.common.empty') }} />
      {list.hasNextPage ? (
        <Button onClick={() => list.fetchNextPage()} loading={list.isFetchingNextPage}>
          {t('chatSafety.common.loadMore')}
        </Button>
      ) : null}
    </Flex>
  );
}

export function SwitchesScreen({ title }: { title: string }) {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const large = useMediaQuery(`(min-width: ${token.screenMD}px)`);
  return (
    <section>
      <Typography.Title level={1}>{title}</Typography.Title>
      {!large ? <Alert type="info" showIcon message={t('chatSafety.common.largeScreen')} /> : null}
      <Tabs
        items={[
          { key: 'state', label: t('chatSafety.switches.tab'), children: <SwitchTable large={large} /> },
          { key: 'history', label: t('chatSafety.switches.history'), children: <History /> },
        ]}
      />
    </section>
  );
}
