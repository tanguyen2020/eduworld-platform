// Screen of `menu.live_ops.tenant_terms` (F122 US6; F122-AC-15, AC-16): the Safety Team works for ONE target tenant
// (X-Tenant-Id on every call, ADR-0072): extra terms of the tenant in existing categories (list/create/delete, cursor
// pages), "Thử chuỗi" with `tenant_id`, publish with a change summary that must carry the tenant's request code; tenant
// categories (listTenantSensitiveCategories of that tenant) with a toggle (reason ≥ 10) only when `allow_tenant_toggle`,
// otherwise a locked switch with a note; `403 TENANT_TOGGLE_NOT_ALLOWED` is shown by code. Terms are never logged or
// stored in the browser (R14, R15).
import { Alert, Button, Flex, Form, Input, Modal, Switch, Table, Tabs, Tag, Tooltip, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { openapiInputProps, openapiRules, OPS_UI_NS, SearchSelect, SensitiveTermTester } from '@eduworld/ops-ui';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { LIMITS, SafetyError } from '../../../shared/chatSafety/ui';
import { PublishDialog } from '../../../shared/chatSafety/PublishDialog';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { ReasonDialog } from '../../../shell/ReasonDialog';
import { useCatalogCategories, useTenantCategories, useTenantTerms, useTenantTermWrite, useTestTerms, useToggleTenantCategory, type SensitiveTerm, type TenantCategory } from './api';
import { formatTime } from './CasesScreen';
import { ModerationTenantFilterSingle } from './SanctionsScreen';

interface TermForm {
  category_id: string;
  term: string;
  match_type: 'word' | 'phrase' | 'pattern';
  locale: 'vi' | 'en';
  note?: string;
}

function Terms({ tenantId }: { tenantId: string }) {
  const { t } = useTranslation();
  const ops = useTranslation(OPS_UI_NS);
  const list = useTenantTerms(tenantId);
  const categories = useCatalogCategories();
  const write = useTenantTermWrite(tenantId);
  const test = useTestTerms();
  const [form] = Form.useForm<TermForm>();
  const [adding, setAdding] = useState(false);
  const [publishing, setPublishing] = useState(false);
  const [tested, setTested] = useState<{ text: string; outcome: { decision: string; masked_text?: string; matches: never[] } } | null>(null);
  // a tenant term goes into an existing category of the platform catalogue (its id comes from listSensitiveCategories)
  const columns: ColumnsType<SensitiveTerm> = [
    { key: 'term', title: t('chatSafety.terms.term'), render: (_, r) => r.term },
    { key: 'match', title: t('chatSafety.terms.matchType'), render: (_, r) => t(`chatSafety.matchType.${r.match_type}`) },
    { key: 'locale', title: t('chatSafety.terms.locale'), render: (_, r) => r.locale },
    { key: 'status', title: t('chatSafety.terms.status'), render: (_, r) => <Tag>{t(`chatSafety.termStatus.${r.status ?? 'draft'}`)}</Tag> },
    { key: 'updated', title: t('chatSafety.terms.updated'), render: (_, r) => formatTime(r.updated_at) },
    {
      key: 'actions',
      title: t('chatSafety.common.actions'),
      render: (_, r) => (
        <ActionGate actionKey="action.live_ops.tenant_terms.edit">
          <Button
            size="small"
            danger
            onClick={() =>
              Modal.confirm({
                title: t('chatSafety.terms.deleteConfirm'),
                okText: t('chatSafety.common.delete'),
                cancelText: t('chatSafety.common.cancel'),
                okButtonProps: { danger: true },
                onOk: () => write.remove.mutateAsync(r.term_id),
              })
            }
          >
            {t('chatSafety.common.delete')}
          </Button>
        </ActionGate>
      ),
    },
  ];
  return (
    <Flex vertical gap="middle">
      <Flex gap="small" wrap>
        <ActionGate actionKey="action.live_ops.tenant_terms.edit">
          <Button type="primary" onClick={() => setAdding(true)}>
            {t('chatSafety.terms.add')}
          </Button>
        </ActionGate>
        <ActionGate actionKey="action.live_ops.tenant_terms.edit">
          <Button onClick={() => setPublishing(true)}>{t('chatSafety.publish.button')}</Button>
        </ActionGate>
      </Flex>
      {list.error ? <SafetyError error={list.error} /> : null}
      {write.remove.error ? <ProblemMessage error={write.remove.error} /> : null}
      <Table rowKey="term_id" columns={columns} dataSource={list.rows} loading={list.isFetching} pagination={false} scroll={{ x: true }} locale={{ emptyText: t('chatSafety.common.empty') }} />
      {list.hasNextPage ? (
        <Button onClick={() => list.fetchNextPage()} loading={list.isFetchingNextPage}>
          {t('chatSafety.common.loadMore')}
        </Button>
      ) : null}
      <Typography.Title level={2}>{t('chatSafety.tester.title')}</Typography.Title>
      <SensitiveTermTester
        maxLength={LIMITS.testText.maxLength}
        loading={test.isPending}
        result={tested}
        error={test.error ? <ProblemMessage error={test.error} /> : null}
        onTest={(text) => test.mutate({ text, tenant_id: tenantId, include_drafts: true }, { onSuccess: (outcome) => setTested({ text, outcome: outcome as never }) })}
      />
      <Modal open={adding} title={t('chatSafety.terms.add')} onCancel={() => setAdding(false)} footer={null} destroyOnHidden>
        <Form
          form={form}
          layout="vertical"
          name="tenant_term"
          initialValues={{ match_type: 'word', locale: 'vi' }}
          onFinish={(v) =>
            write.create.mutate(
              { category_id: v.category_id, term: v.term.trim(), match_type: v.match_type, locale: v.locale, enabled: true, ...(v.note?.trim() ? { note: v.note.trim() } : {}) },
              { onSuccess: () => setAdding(false) },
            )
          }
        >
          <Form.Item name="category_id" label={t('chatSafety.terms.category')} rules={[{ required: true, message: t('chatSafety.terms.categoryRequired') }]}>
            <SearchSelect aria-label={t('chatSafety.terms.category')} options={categories.items.map((c) => ({ value: c.category_id, label: c.name }))} truncated={categories.truncated} />
          </Form.Item>
          <Form.Item name="term" label={t('chatSafety.terms.term')} rules={openapiRules(LIMITS.term, { required: true, t: ops.t })}>
            <Input {...openapiInputProps(LIMITS.term)} autoComplete="off" />
          </Form.Item>
          <Form.Item name="match_type" label={t('chatSafety.terms.matchType')}>
            <SearchSelect aria-label={t('chatSafety.terms.matchType')} options={(['word', 'phrase', 'pattern'] as const).map((v) => ({ value: v, label: t(`chatSafety.matchType.${v}`) }))} />
          </Form.Item>
          <Form.Item name="locale" label={t('chatSafety.terms.locale')}>
            <SearchSelect aria-label={t('chatSafety.terms.locale')} options={(['vi', 'en'] as const).map((v) => ({ value: v, label: v }))} />
          </Form.Item>
          <Form.Item name="note" label={t('chatSafety.terms.note')} rules={openapiRules(LIMITS.termNote, { t: ops.t })}>
            <Input.TextArea autoSize={{ minRows: 2, maxRows: 4 }} {...openapiInputProps(LIMITS.termNote)} />
          </Form.Item>
          {write.create.error ? <ProblemMessage error={write.create.error} /> : null}
          <Button type="primary" htmlType="submit" loading={write.create.isPending}>
            {t('chatSafety.common.save')}
          </Button>
        </Form>
      </Modal>
      <PublishDialog
        open={publishing}
        requireRequestCode
        loading={write.publish.isPending}
        error={write.publish.error}
        onCancel={() => setPublishing(false)}
        onSubmit={(summary) => write.publish.mutate(summary, { onSuccess: () => setPublishing(false) })}
      />
    </Flex>
  );
}

function Categories({ tenantId, list }: { tenantId: string; list: ReturnType<typeof useTenantCategories> }) {
  const { t } = useTranslation();
  const toggle = useToggleTenantCategory(tenantId);
  const [target, setTarget] = useState<{ code: string; enabled: boolean } | null>(null);
  const columns: ColumnsType<TenantCategory> = [
    { key: 'name', title: t('chatSafety.categories.name'), render: (_, r) => r.name },
    { key: 'severity', title: t('chatSafety.categories.severity'), render: (_, r) => t(`chatSafety.termSeverity.${r.severity}`) },
    { key: 'platform', title: t('chatSafety.categories.platform'), render: (_, r) => (r.enabled_platform ? t('chatSafety.common.on') : t('chatSafety.common.off')) },
    {
      key: 'tenant',
      title: t('chatSafety.categories.inTenant'),
      render: (_, r) =>
        r.allow_tenant_toggle ? (
          <ActionGate actionKey="action.live_ops.tenant_terms.toggle_category" mode="disable">
            <Switch aria-label={t('chatSafety.categories.toggle', { name: r.name })} checked={r.enabled_in_tenant} onChange={(on) => setTarget({ code: r.code, enabled: on })} />
          </ActionGate>
        ) : (
          <Tooltip title={t('chatSafety.categories.locked')}>
            <Flex align="center" gap="small">
              <Switch aria-label={t('chatSafety.categories.toggle', { name: r.name })} checked={r.enabled_in_tenant} disabled />
              <Typography.Text type="secondary">{t('chatSafety.categories.locked')}</Typography.Text>
            </Flex>
          </Tooltip>
        ),
    },
  ];
  return (
    <Flex vertical gap="middle">
      {list.error ? <SafetyError error={list.error} /> : null}
      {toggle.error ? <ProblemMessage error={toggle.error} /> : null}
      <Table rowKey="code" columns={columns} dataSource={list.rows} loading={list.isFetching} pagination={false} scroll={{ x: true }} />
      {list.hasNextPage ? (
        <Button onClick={() => list.fetchNextPage()} loading={list.isFetchingNextPage}>
          {t('chatSafety.common.loadMore')}
        </Button>
      ) : null}
      <ReasonDialog
        open={!!target}
        title={target?.enabled ? t('chatSafety.categories.enable') : t('chatSafety.categories.disable')}
        maxLength={LIMITS.toggleReason.maxLength}
        minLength={LIMITS.toggleReason.minLength}
        confirmLoading={toggle.isPending}
        onCancel={() => setTarget(null)}
        onSubmit={(reason) => target && toggle.mutate({ ...target, reason }, { onSettled: () => setTarget(null) })}
      />
    </Flex>
  );
}

export function TenantTermsScreen({ title }: { title: string }) {
  const { t } = useTranslation();
  const [tenantId, setTenantId] = useState<string | undefined>();
  const categories = useTenantCategories(tenantId);
  return (
    <section>
      <Typography.Title level={1}>{title}</Typography.Title>
      <Form layout="inline" name="tenant_terms_target">
        <Form.Item label={t('chatSafety.common.targetTenant')}>
          <ModerationTenantFilterSingle value={tenantId} onChange={setTenantId} />
        </Form.Item>
      </Form>
      {!tenantId ? (
        <Alert type="info" showIcon message={t('chatSafety.terms.pickTenant')} />
      ) : (
        <Tabs
          items={[
            { key: 'terms', label: t('chatSafety.terms.tab'), children: <Terms tenantId={tenantId} /> },
            { key: 'categories', label: t('chatSafety.categories.tab'), children: <Categories tenantId={tenantId} list={categories} /> },
          ]}
        />
      )}
    </section>
  );
}
