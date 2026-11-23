// Screen of `menu.platform_admin.chat_safety` (F122 US3; F122-AC-07, AC-08, AC-09): the platform sensitive-term catalogue
// — categories (create/edit; delete = soft delete after confirmation), terms (category / locale / match type / status
// filters applied by the server in SQL — F122 T063 — cursor pages,
// variants ≤ 50, RE2 warning for `pattern`), allowlist, "Thử chuỗi" (testSensitiveTerms, optional tenant) and publish
// (mandatory change summary) with the version list and its regression figures. Without the edit/test/publish actions the
// catalogue is read only (ActionGate) — the Safety Moderator (chat.sensitive_terms.read) opens it read only (T062). On a phone the screen is read only ("cần màn hình lớn hơn", ADMIN_PORTAL §1.5).
// Terms are never written to logs, telemetry or browser storage (R14, R15).
import { Alert, Button, Flex, Form, Input, Modal, Switch, Table, Tabs, Tag, Typography, theme } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { openapiInputProps, openapiRules, OPS_UI_NS, SearchSelect, SensitiveTermTester } from '@eduworld/ops-ui';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { LIMITS, SafetyError } from '../../../shared/chatSafety/ui';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { useMediaQuery } from '../../../shell/useMediaQuery';
import { PublishDialog } from '../../../shared/chatSafety/PublishDialog';
import { formatTime } from '../ui';
import { useAllCategories, useAllowlist, useCatalogWrites, useCategories, useTerms, useTestTerms, useVersions, type Category, type CategoryInput, type Term, type TermFilters } from './api';

const SEVERITIES = ['low', 'medium', 'high', 'critical'] as const;
const ACTIONS = ['allow', 'mask', 'block', 'hold_for_review', 'block_and_alert'] as const;
const MATCH_TYPES = ['word', 'phrase', 'pattern'] as const;

function useLarge(): boolean {
  const { token } = theme.useToken();
  return useMediaQuery(`(min-width: ${token.screenMD}px)`);
}

type Writes = ReturnType<typeof useCatalogWrites>;

function CategoryDialog({ editing, writes, onClose }: { editing: Category | 'new'; writes: Writes; onClose: () => void }) {
  const { t } = useTranslation();
  const ops = useTranslation(OPS_UI_NS);
  const [form] = Form.useForm<CategoryInput>();
  const initial = editing === 'new' ? { severity: 'medium', default_action: 'mask', enabled: true, allow_tenant_toggle: false } : editing;
  const mutation = editing === 'new' ? writes.createCategory : writes.updateCategory;
  return (
    <Modal open title={editing === 'new' ? t('chatSafety.categories.create') : t('chatSafety.categories.edit')} onCancel={onClose} footer={null} destroyOnHidden>
      <Form
        form={form}
        layout="vertical"
        name="category"
        initialValues={initial}
        onFinish={(v) => {
          const body: CategoryInput = { ...v, name: v.name.trim(), ...(v.description?.trim() ? { description: v.description.trim() } : {}) };
          if (editing === 'new') writes.createCategory.mutate(body, { onSuccess: onClose });
          else writes.updateCategory.mutate({ id: editing.category_id, body }, { onSuccess: onClose });
        }}
      >
        <Form.Item name="code" label={t('chatSafety.categories.code')} rules={openapiRules(LIMITS.categoryCode, { required: true, t: ops.t })}>
          <Input {...openapiInputProps(LIMITS.categoryCode)} disabled={editing !== 'new'} />
        </Form.Item>
        <Form.Item name="name" label={t('chatSafety.categories.name')} rules={openapiRules(LIMITS.categoryName, { required: true, t: ops.t })}>
          <Input {...openapiInputProps(LIMITS.categoryName)} />
        </Form.Item>
        <Form.Item name="description" label={t('chatSafety.categories.description')} rules={openapiRules(LIMITS.categoryDescription, { t: ops.t })}>
          <Input.TextArea autoSize={{ minRows: 2, maxRows: 5 }} {...openapiInputProps(LIMITS.categoryDescription)} />
        </Form.Item>
        <Form.Item name="severity" label={t('chatSafety.categories.severity')}>
          <SearchSelect aria-label={t('chatSafety.categories.severity')} options={SEVERITIES.map((s) => ({ value: s, label: t(`chatSafety.termSeverity.${s}`) }))} />
        </Form.Item>
        <Form.Item name="default_action" label={t('chatSafety.categories.defaultAction')}>
          <SearchSelect aria-label={t('chatSafety.categories.defaultAction')} options={ACTIONS.map((a) => ({ value: a, label: t(`chatSafety.decision.${a}`) }))} />
        </Form.Item>
        <Form.Item name="enabled" label={t('chatSafety.categories.enabled')} valuePropName="checked">
          <Switch />
        </Form.Item>
        <Form.Item name="allow_tenant_toggle" label={t('chatSafety.categories.allowTenantToggle')} valuePropName="checked">
          <Switch />
        </Form.Item>
        {mutation.error ? <ProblemMessage error={mutation.error} /> : null}
        <Button type="primary" htmlType="submit" loading={mutation.isPending}>
          {t('chatSafety.common.save')}
        </Button>
      </Form>
    </Modal>
  );
}

function Categories({ large, writes }: { large: boolean; writes: Writes }) {
  const { t } = useTranslation();
  const list = useCategories();
  const [editing, setEditing] = useState<Category | 'new' | null>(null);
  const columns: ColumnsType<Category> = [
    { key: 'code', title: t('chatSafety.categories.code'), render: (_, r) => <Typography.Text code>{r.code}</Typography.Text> },
    { key: 'name', title: t('chatSafety.categories.name'), render: (_, r) => r.name },
    { key: 'severity', title: t('chatSafety.categories.severity'), render: (_, r) => t(`chatSafety.termSeverity.${r.severity}`) },
    { key: 'action', title: t('chatSafety.categories.defaultAction'), render: (_, r) => t(`chatSafety.decision.${r.default_action}`) },
    { key: 'enabled', title: t('chatSafety.categories.enabled'), render: (_, r) => (r.enabled ? t('chatSafety.common.on') : t('chatSafety.common.off')) },
    { key: 'toggle', title: t('chatSafety.categories.allowTenantToggle'), render: (_, r) => (r.allow_tenant_toggle ? t('chatSafety.common.yes') : t('chatSafety.common.no')) },
    { key: 'count', title: t('chatSafety.categories.termCount'), render: (_, r) => r.term_count ?? 0 },
    {
      key: 'actions',
      title: t('chatSafety.common.actions'),
      render: (_, r) =>
        large ? (
          <ActionGate actionKey="action.platform_admin.chat_safety.edit">
            <Flex gap="small">
              <Button size="small" onClick={() => setEditing(r)}>
                {t('chatSafety.common.edit')}
              </Button>
              <Button
                size="small"
                danger
                onClick={() =>
                  Modal.confirm({
                    title: t('chatSafety.categories.deleteConfirm', { name: r.name }),
                    content: t('chatSafety.categories.deleteHelp'),
                    okText: t('chatSafety.common.delete'),
                    cancelText: t('chatSafety.common.cancel'),
                    okButtonProps: { danger: true },
                    onOk: () => writes.deleteCategory.mutateAsync(r.category_id),
                  })
                }
              >
                {t('chatSafety.common.delete')}
              </Button>
            </Flex>
          </ActionGate>
        ) : null,
    },
  ];
  return (
    <Flex vertical gap="middle">
      {large ? (
        <ActionGate actionKey="action.platform_admin.chat_safety.edit">
          <Button type="primary" onClick={() => setEditing('new')} style={{ alignSelf: 'flex-start' }}>
            {t('chatSafety.categories.create')}
          </Button>
        </ActionGate>
      ) : null}
      {list.error ? <SafetyError error={list.error} /> : null}
      {writes.deleteCategory.error ? <ProblemMessage error={writes.deleteCategory.error} /> : null}
      <Table rowKey="category_id" columns={columns} dataSource={list.rows} loading={list.isFetching} pagination={false} scroll={{ x: true }} />
      {list.hasNextPage ? (
        <Button onClick={() => list.fetchNextPage()} loading={list.isFetchingNextPage}>
          {t('chatSafety.common.loadMore')}
        </Button>
      ) : null}
      {editing ? <CategoryDialog editing={editing} writes={writes} onClose={() => setEditing(null)} /> : null}
    </Flex>
  );
}

interface TermForm {
  category_id: string;
  term: string;
  match_type: (typeof MATCH_TYPES)[number];
  locale: 'vi' | 'en';
  variants?: string[];
  enabled: boolean;
  note?: string;
}

function Terms({ large, writes }: { large: boolean; writes: Writes }) {
  const { t } = useTranslation();
  const ops = useTranslation(OPS_UI_NS);
  const categories = useAllCategories();
  const [filters, setFilters] = useState<TermFilters>({});
  const set = (patch: Partial<TermFilters>) => setFilters((f) => ({ ...f, ...patch }));
  // every filter is a query parameter applied by module chat in SQL (keyset kept, F122 T063)
  const list = useTerms(filters);
  const [adding, setAdding] = useState(false);
  const [form] = Form.useForm<TermForm>();
  const matchType = Form.useWatch('match_type', form);
  const names = new Map(categories.items.map((c) => [c.category_id, c.name]));
  const columns: ColumnsType<Term> = [
    { key: 'term', title: t('chatSafety.terms.term'), render: (_, r) => r.term },
    { key: 'category', title: t('chatSafety.terms.category'), render: (_, r) => names.get(r.category_id) ?? '' },
    { key: 'match', title: t('chatSafety.terms.matchType'), render: (_, r) => t(`chatSafety.matchType.${r.match_type}`) },
    { key: 'locale', title: t('chatSafety.terms.locale'), render: (_, r) => r.locale },
    { key: 'variants', title: t('chatSafety.terms.variants'), render: (_, r) => r.variants?.length ?? 0 },
    { key: 'status', title: t('chatSafety.terms.status'), render: (_, r) => <Tag>{t(`chatSafety.termStatus.${r.status ?? 'draft'}`)}</Tag> },
    { key: 'enabled', title: t('chatSafety.categories.enabled'), render: (_, r) => (r.enabled ? t('chatSafety.common.on') : t('chatSafety.common.off')) },
    {
      key: 'actions',
      title: t('chatSafety.common.actions'),
      render: (_, r) =>
        large ? (
          <ActionGate actionKey="action.platform_admin.chat_safety.edit">
            <Button
              size="small"
              danger
              onClick={() =>
                Modal.confirm({
                  title: t('chatSafety.terms.deleteConfirm'),
                  okText: t('chatSafety.common.delete'),
                  cancelText: t('chatSafety.common.cancel'),
                  okButtonProps: { danger: true },
                  onOk: () => writes.deleteTerm.mutateAsync(r.term_id),
                })
              }
            >
              {t('chatSafety.common.delete')}
            </Button>
          </ActionGate>
        ) : null,
    },
  ];
  return (
    <Flex vertical gap="middle">
      <Flex gap="small" wrap align="center">
        <SearchSelect
          aria-label={t('chatSafety.terms.category')}
          placeholder={t('chatSafety.terms.allCategories')}
          allowClear
          value={filters.categoryId}
          onChange={(v) => set({ categoryId: v })}
          options={categories.items.map((c) => ({ value: c.category_id, label: c.name }))}
          style={{ minWidth: '14rem' }}
        />
        <SearchSelect
          aria-label={t('chatSafety.terms.locale')}
          placeholder={t('chatSafety.terms.allLocales')}
          allowClear
          value={filters.locale}
          onChange={(v) => set({ locale: v })}
          options={(['vi', 'en'] as const).map((v) => ({ value: v, label: v }))}
          style={{ minWidth: '8rem' }}
        />
        <SearchSelect
          aria-label={t('chatSafety.terms.matchType')}
          placeholder={t('chatSafety.terms.allMatchTypes')}
          allowClear
          value={filters.matchType}
          onChange={(v) => set({ matchType: v })}
          options={MATCH_TYPES.map((v) => ({ value: v, label: t(`chatSafety.matchType.${v}`) }))}
          style={{ minWidth: '9rem' }}
        />
        <SearchSelect
          aria-label={t('chatSafety.terms.status')}
          placeholder={t('chatSafety.terms.allStatuses')}
          allowClear
          value={filters.status}
          onChange={(v) => set({ status: v })}
          options={(['draft', 'published'] as const).map((v) => ({ value: v, label: t(`chatSafety.termStatus.${v}`) }))}
          style={{ minWidth: '9rem' }}
        />
        {large ? (
          <ActionGate actionKey="action.platform_admin.chat_safety.edit">
            <Button type="primary" onClick={() => setAdding(true)}>
              {t('chatSafety.terms.add')}
            </Button>
          </ActionGate>
        ) : null}
      </Flex>
      {list.error ? <SafetyError error={list.error} /> : null}
      <Table rowKey="term_id" columns={columns} dataSource={list.rows} loading={list.isFetching} pagination={false} scroll={{ x: true }} locale={{ emptyText: t('chatSafety.common.empty') }} />
      {list.hasNextPage ? (
        <Button onClick={() => list.fetchNextPage()} loading={list.isFetchingNextPage}>
          {t('chatSafety.common.loadMore')}
        </Button>
      ) : null}
      <Modal open={adding} title={t('chatSafety.terms.add')} onCancel={() => setAdding(false)} footer={null} destroyOnHidden>
        <Form
          form={form}
          layout="vertical"
          name="platform_term"
          initialValues={{ match_type: 'word', locale: 'vi', enabled: true }}
          onFinish={(v) =>
            writes.createTerm.mutate(
              {
                category_id: v.category_id,
                term: v.term.trim(),
                match_type: v.match_type,
                locale: v.locale,
                enabled: v.enabled,
                ...(v.variants?.length ? { variants: v.variants.map((x) => x.trim()).filter(Boolean) } : {}),
                ...(v.note?.trim() ? { note: v.note.trim() } : {}),
              },
              { onSuccess: () => setAdding(false) },
            )
          }
        >
          <Form.Item name="category_id" label={t('chatSafety.terms.category')} rules={[{ required: true, message: t('chatSafety.terms.categoryRequired') }]}>
            <SearchSelect aria-label={t('chatSafety.terms.category')} options={categories.items.map((c) => ({ value: c.category_id, label: c.name }))} />
          </Form.Item>
          <Form.Item name="term" label={t('chatSafety.terms.term')} rules={openapiRules(LIMITS.term, { required: true, t: ops.t })}>
            <Input {...openapiInputProps(LIMITS.term)} autoComplete="off" />
          </Form.Item>
          <Form.Item name="match_type" label={t('chatSafety.terms.matchType')}>
            <SearchSelect aria-label={t('chatSafety.terms.matchType')} options={MATCH_TYPES.map((v) => ({ value: v, label: t(`chatSafety.matchType.${v}`) }))} />
          </Form.Item>
          {matchType === 'pattern' ? <Alert type="warning" showIcon message={t('chatSafety.terms.re2Warning')} /> : null}
          <Form.Item name="locale" label={t('chatSafety.terms.locale')}>
            <SearchSelect aria-label={t('chatSafety.terms.locale')} options={(['vi', 'en'] as const).map((v) => ({ value: v, label: v }))} />
          </Form.Item>
          <Form.Item
            name="variants"
            label={t('chatSafety.terms.variants')}
            extra={t('chatSafety.terms.variantsHelp', { max: 50 })}
            rules={[{ type: 'array', max: 50, message: t('chatSafety.terms.variantsMax', { max: 50 }) }]}
          >
            <SearchSelect aria-label={t('chatSafety.terms.variants')} mode="tags" maxCount={50} open={false} tokenSeparators={[',']} maxTagTextLength={LIMITS.term.maxLength} />
          </Form.Item>
          <Form.Item name="enabled" label={t('chatSafety.categories.enabled')} valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="note" label={t('chatSafety.terms.note')} rules={openapiRules(LIMITS.termNote, { t: ops.t })}>
            <Input.TextArea autoSize={{ minRows: 2, maxRows: 4 }} {...openapiInputProps(LIMITS.termNote)} />
          </Form.Item>
          {writes.createTerm.error ? <ProblemMessage error={writes.createTerm.error} /> : null}
          <Button type="primary" htmlType="submit" loading={writes.createTerm.isPending}>
            {t('chatSafety.common.save')}
          </Button>
        </Form>
      </Modal>
    </Flex>
  );
}

function Allowlist({ large, writes }: { large: boolean; writes: Writes }) {
  const { t } = useTranslation();
  const ops = useTranslation(OPS_UI_NS);
  const list = useAllowlist();
  const [adding, setAdding] = useState(false);
  const columns: ColumnsType<{ entry_id: string; phrase: string; locale: string; reason?: string }> = [
    { key: 'phrase', title: t('chatSafety.allowlist.phrase'), render: (_, r) => r.phrase },
    { key: 'locale', title: t('chatSafety.terms.locale'), render: (_, r) => r.locale },
    { key: 'reason', title: t('chatSafety.allowlist.reason'), render: (_, r) => r.reason ?? '' },
    {
      key: 'actions',
      title: t('chatSafety.common.actions'),
      render: (_, r) =>
        large ? (
          <ActionGate actionKey="action.platform_admin.chat_safety.edit">
            <Button size="small" danger onClick={() => writes.deleteAllow.mutate(r.entry_id)}>
              {t('chatSafety.common.delete')}
            </Button>
          </ActionGate>
        ) : null,
    },
  ];
  return (
    <Flex vertical gap="middle">
      {large ? (
        <ActionGate actionKey="action.platform_admin.chat_safety.edit">
          <Button type="primary" onClick={() => setAdding(true)} style={{ alignSelf: 'flex-start' }}>
            {t('chatSafety.allowlist.add')}
          </Button>
        </ActionGate>
      ) : null}
      {list.error ? <SafetyError error={list.error} /> : null}
      <Table rowKey="entry_id" columns={columns} dataSource={list.rows} loading={list.isFetching} pagination={false} scroll={{ x: true }} locale={{ emptyText: t('chatSafety.common.empty') }} />
      {list.hasNextPage ? (
        <Button onClick={() => list.fetchNextPage()} loading={list.isFetchingNextPage}>
          {t('chatSafety.common.loadMore')}
        </Button>
      ) : null}
      <Modal open={adding} title={t('chatSafety.allowlist.add')} onCancel={() => setAdding(false)} footer={null} destroyOnHidden>
        <Form
          layout="vertical"
          name="allowlist"
          initialValues={{ locale: 'vi' }}
          onFinish={(v: { phrase: string; locale: 'vi' | 'en'; reason: string }) =>
            writes.createAllow.mutate({ phrase: v.phrase.trim(), locale: v.locale, reason: v.reason.trim() }, { onSuccess: () => setAdding(false) })
          }
        >
          <Form.Item name="phrase" label={t('chatSafety.allowlist.phrase')} rules={openapiRules(LIMITS.allowPhrase, { required: true, t: ops.t })}>
            <Input {...openapiInputProps(LIMITS.allowPhrase)} autoComplete="off" />
          </Form.Item>
          <Form.Item name="locale" label={t('chatSafety.terms.locale')}>
            <SearchSelect aria-label={t('chatSafety.terms.locale')} options={(['vi', 'en'] as const).map((v) => ({ value: v, label: v }))} />
          </Form.Item>
          <Form.Item name="reason" label={t('chatSafety.allowlist.reason')} rules={openapiRules(LIMITS.allowReason, { required: true, t: ops.t })}>
            <Input.TextArea autoSize={{ minRows: 2, maxRows: 4 }} {...openapiInputProps(LIMITS.allowReason)} />
          </Form.Item>
          {writes.createAllow.error ? <ProblemMessage error={writes.createAllow.error} /> : null}
          <Button type="primary" htmlType="submit" loading={writes.createAllow.isPending}>
            {t('chatSafety.common.save')}
          </Button>
        </Form>
      </Modal>
    </Flex>
  );
}

function Tester() {
  const test = useTestTerms();
  const [result, setResult] = useState<{ text: string; outcome: { decision: string; masked_text?: string; matches: { category_code: string; severity: string; action: string; start: number; end: number }[] } } | null>(null);
  return (
    <ActionGate actionKey="action.platform_admin.chat_safety.test">
      <SensitiveTermTester
        maxLength={LIMITS.testText.maxLength}
        loading={test.isPending}
        result={result}
        error={test.error ? <ProblemMessage error={test.error} /> : null}
        onTest={(text) => test.mutate({ text, include_drafts: true }, { onSuccess: (outcome) => setResult({ text, outcome }) })}
      />
    </ActionGate>
  );
}

function Versions({ large, writes }: { large: boolean; writes: Writes }) {
  const { t } = useTranslation();
  const list = useVersions();
  const [publishing, setPublishing] = useState(false);
  const pct = (v?: number) => (v == null ? '' : `${(v * 100).toFixed(1)}%`);
  const columns: ColumnsType<{ catalog_version: number; published_at: string; change_summary?: string; term_count: number; regression?: { recall?: number; false_positive_rate?: number } }> = [
    { key: 'version', title: t('chatSafety.versions.version'), render: (_, r) => r.catalog_version },
    { key: 'at', title: t('chatSafety.versions.publishedAt'), render: (_, r) => formatTime(r.published_at) },
    { key: 'summary', title: t('chatSafety.publish.summary'), render: (_, r) => r.change_summary ?? '' },
    { key: 'terms', title: t('chatSafety.categories.termCount'), render: (_, r) => r.term_count },
    { key: 'recall', title: t('chatSafety.versions.recall'), render: (_, r) => pct(r.regression?.recall) },
    { key: 'fpr', title: t('chatSafety.versions.falsePositive'), render: (_, r) => pct(r.regression?.false_positive_rate) },
  ];
  return (
    <Flex vertical gap="middle">
      {large ? (
        <ActionGate actionKey="action.platform_admin.chat_safety.publish">
          <Button type="primary" onClick={() => setPublishing(true)} style={{ alignSelf: 'flex-start' }}>
            {t('chatSafety.publish.button')}
          </Button>
        </ActionGate>
      ) : null}
      {list.error ? <SafetyError error={list.error} /> : null}
      <Table rowKey="catalog_version" columns={columns} dataSource={list.rows} loading={list.isFetching} pagination={false} scroll={{ x: true }} />
      {list.hasNextPage ? (
        <Button onClick={() => list.fetchNextPage()} loading={list.isFetchingNextPage}>
          {t('chatSafety.common.loadMore')}
        </Button>
      ) : null}
      <PublishDialog
        open={publishing}
        loading={writes.publish.isPending}
        error={writes.publish.error}
        onCancel={() => setPublishing(false)}
        onSubmit={(summary) => writes.publish.mutate(summary, { onSuccess: () => setPublishing(false) })}
      />
    </Flex>
  );
}

export function CatalogScreen({ title }: { title: string }) {
  const { t } = useTranslation();
  const large = useLarge();
  const writes = useCatalogWrites();
  return (
    <section>
      <Typography.Title level={1}>{title}</Typography.Title>
      {!large ? <Alert type="info" showIcon message={t('chatSafety.common.largeScreen')} /> : null}
      <Tabs
        items={[
          { key: 'categories', label: t('chatSafety.categories.tab'), children: <Categories large={large} writes={writes} /> },
          { key: 'terms', label: t('chatSafety.terms.tab'), children: <Terms large={large} writes={writes} /> },
          { key: 'allowlist', label: t('chatSafety.allowlist.tab'), children: <Allowlist large={large} writes={writes} /> },
          { key: 'test', label: t('chatSafety.tester.title'), children: <Tester /> },
          { key: 'versions', label: t('chatSafety.versions.tab'), children: <Versions large={large} writes={writes} /> },
        ]}
      />
    </section>
  );
}
