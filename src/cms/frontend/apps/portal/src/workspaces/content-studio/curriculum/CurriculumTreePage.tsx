// CurriculumTreePage — menu `menu.content_studio.curriculum` (F053 US4: F053-AC-26…AC-30, CURRICULUM_MODEL §2,
// ADMIN_PORTAL §3.1): pick a curriculum and one of its versions, see Môn → môn × khối → chủ đề → LO, drag topics,
// add topics and LOs, edit an LO, run the version workflow. Platform curricula assigned to a tenant are read only for
// tenant users ("Từ EduWorld"); only `curriculum.write` in a tenant scope (Academic Manager) creates the tenant's own
// curriculum. Editing needs a draft version. Display only: the backend authorizes (AGENTS R20).
import { Button, Card, Col, Empty, Flex, Form, Input, Modal, Row, Select, Space, theme, Typography } from 'antd';
import { useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { useTenantSource } from '../../../shared/auth/TenantContext';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { useSession } from '../../../session/SessionProvider';
import { useContentAccess } from '../shared/access';
import { AssignedBadge } from '../shared/AssignedBadge';
import { LIMITS } from '../shared/config';
import { OWNER_FIELD, ownerOf, OwnerScopeField } from '../shared/OwnerScopeField';
import { CurriculumVersionPanel } from './CurriculumVersionPanel';
import { ObjectiveForm } from './ObjectiveForm';
import { TopicTree } from './TopicTree';
import type { TreeNode } from './treeModel';
import { useCurricula, useCurriculumApi, useCurriculumVersions, useObjectives, useSubjects, useTopics, type Curriculum, type CurriculumVersion } from './useCurriculum';

const nameOf = (n: { vi?: string; en?: string } | undefined, lang: string) => (lang.startsWith('en') ? (n?.en ?? n?.vi) : (n?.vi ?? n?.en)) ?? '';

export function CurriculumTreePage({ title }: { title: string }) {
  const { t, i18n } = useTranslation();
  const { token } = theme.useToken();
  const access = useContentAccess();
  const tenant = useTenantSource();
  const { scopeKey } = useSession();
  const qc = useQueryClient();
  const api = useCurriculumApi();
  const curricula = useCurricula();
  const subjects = useSubjects();
  const [curriculumId, setCurriculumId] = useState<string>();
  const versions = useCurriculumVersions(curriculumId);
  const [versionNo, setVersionNo] = useState<number>();
  const [loaded, setLoaded] = useState<{ data: CurriculumVersion; etag: string | null } | null>(null);
  const [loadError, setLoadError] = useState<unknown>(null);
  const [selected, setSelected] = useState<TreeNode | null>(null);
  const [creating, setCreating] = useState<'curriculum' | 'topic' | null>(null);

  const curriculum: Curriculum | undefined = curricula.data?.data.find((c) => c.curriculum_id === curriculumId);
  useEffect(() => {
    if (!curriculumId && curricula.data?.data[0]) setCurriculumId(curricula.data.data[0].curriculum_id);
  }, [curricula.data, curriculumId]);
  useEffect(() => {
    const list = versions.data?.data ?? [];
    if (list.length && !list.some((v) => v.version === versionNo)) setVersionNo(Math.max(...list.map((v) => v.version)));
  }, [versions.data, versionNo]);
  useEffect(() => {
    if (!curriculumId || !versionNo) return;
    let live = true;
    setLoadError(null);
    api.loadVersion(curriculumId, versionNo).then(
      (r) => live && setLoaded(r),
      (e: unknown) => live && setLoadError(e),
    );
    return () => {
      live = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [curriculumId, versionNo]);

  const version = loaded?.data.curriculum_id === curriculumId ? loaded?.data : undefined;
  const vid = version?.curriculum_version_id;
  const topics = useTopics({ curriculum_version_id: vid }, !!vid);
  const objectives = useObjectives({ curriculum_version_id: vid }, !!vid);

  // tenant users never edit platform curricula (assigned → "Từ EduWorld", read only)
  const inTenant = !!tenant.getTenantId();
  const assigned = !!curriculum && curriculum.owner_scope === 'platform' && inTenant;
  const editable = access.curriculum.write && !assigned && version?.status === 'draft';
  const subjectName = useMemo(() => {
    const map = new Map((subjects.data?.data ?? []).map((s) => [s.subject_id, nameOf(s.name, i18n.language)]));
    return (id: string) => map.get(id) ?? id;
  }, [subjects.data, i18n.language]);

  const refresh = () => {
    void qc.invalidateQueries({ queryKey: ['content-studio', 'topics', scopeKey] });
    void qc.invalidateQueries({ queryKey: ['content-studio', 'objectives', scopeKey] });
  };
  const reloadVersion = async () => {
    if (curriculumId && versionNo) setLoaded(await api.loadVersion(curriculumId, versionNo));
    void qc.invalidateQueries({ queryKey: ['content-studio', 'curriculum-versions', scopeKey] });
  };

  const selectedTopicId = selected?.kind === 'topic' ? selected.topic?.topic_id : selected?.kind === 'objective' ? selected.objective?.topic_id : undefined;

  return (
    <section aria-labelledby="ew-cs-curriculum">
      <Flex vertical gap="middle">
        <Flex justify="space-between" align="center" wrap gap="small">
          <Typography.Title level={1} id="ew-cs-curriculum">
            {title}
          </Typography.Title>
          {access.curriculum.write && (inTenant || access.crossTenant) && (
            <ActionGate actionKey="action.content_studio.curriculum.edit">
              <Button onClick={() => setCreating('curriculum')}>{t('content_studio.curriculum.createOwn')}</Button>
            </ActionGate>
          )}
        </Flex>
        {curricula.error ? <ProblemMessage error={curricula.error} /> : null}
        <Space wrap>
          <Select
            aria-label={t('content_studio.curriculum.pick')}
            style={{ minWidth: token.controlHeightLG * 8 }}
            loading={curricula.isFetching}
            value={curriculumId}
            onChange={(v) => {
              setCurriculumId(v);
              setVersionNo(undefined);
              setSelected(null);
            }}
            options={(curricula.data?.data ?? []).map((c) => ({
              value: c.curriculum_id,
              label: `${nameOf(c.name, i18n.language)} (${c.code}) · ${t(`content_studio.owner.${c.owner_scope}`)}`,
            }))}
          />
          <Select
            aria-label={t('content_studio.curriculum.pickVersion')}
            value={versionNo}
            onChange={setVersionNo}
            options={(versions.data?.data ?? []).map((v) => ({ value: v.version, label: `${v.version_label} · ${t(`content_studio.status.${v.status}`)}` }))}
          />
          {assigned && <AssignedBadge />}
        </Space>
        {loadError ? <ProblemMessage error={loadError} /> : null}
        {version && (
          <CurriculumVersionPanel
            version={version}
            can={{ write: access.curriculum.write, publish: access.curriculum.publish }}
            readOnly={assigned}
            onTransition={async (action, note) => {
              await api.transitionVersion.mutateAsync({ curriculumId: version.curriculum_id, version: version.version, action, note, etag: loaded?.etag ?? null });
              await reloadVersion();
            }}
            onPublish={async () => {
              await api.publishVersion.mutateAsync({ curriculumId: version.curriculum_id, version: version.version, etag: loaded?.etag ?? null });
              await reloadVersion();
            }}
          />
        )}
        <Row gutter={[token.margin, token.margin]}>
          <Col xs={24} lg={14}>
            <Card
              title={t('content_studio.curriculum.tree')}
              extra={
                editable && (
                  <ActionGate actionKey="action.content_studio.curriculum.edit">
                    <Button size="small" disabled={!selected || selected.kind === 'subject' || selected.kind === 'objective'} onClick={() => setCreating('topic')}>
                      {t('content_studio.curriculum.addTopic')}
                    </Button>
                  </ActionGate>
                )
              }
            >
              {topics.error ? <ProblemMessage error={topics.error} /> : null}
              {version ? (
                <TopicTree
                  subjectGrades={version.subject_grades ?? []}
                  subjectName={subjectName}
                  topics={topics.data?.data ?? []}
                  objectives={objectives.data?.data ?? []}
                  editable={editable}
                  onMove={async (topic, patch) => {
                    await api.updateTopic(topic, patch);
                    refresh();
                  }}
                  onSelect={setSelected}
                />
              ) : (
                <Empty description={t('content_studio.curriculum.pickHint')} />
              )}
            </Card>
          </Col>
          <Col xs={24} lg={10}>
            <Card title={selected?.kind === 'objective' ? t('content_studio.curriculum.objective') : t('content_studio.curriculum.newObjective')}>
              {selectedTopicId ? (
                <ObjectiveForm
                  objective={selected?.kind === 'objective' ? selected.objective : undefined}
                  topicId={selectedTopicId}
                  candidates={objectives.data?.data ?? []}
                  readOnly={!editable}
                  onSubmit={async (input) => {
                    if (selected?.kind === 'objective' && selected.objective) {
                      const { topic_id: _t, ...patch } = input;
                      void _t;
                      await api.updateObjective(selected.objective, patch);
                    } else await api.createObjective.mutateAsync(input);
                    refresh();
                  }}
                />
              ) : (
                <Typography.Text type="secondary">{t('content_studio.curriculum.selectTopic')}</Typography.Text>
              )}
            </Card>
          </Col>
        </Row>
      </Flex>
      <CreateTopicDialog
        open={creating === 'topic'}
        onClose={() => setCreating(null)}
        onCreate={async (code, name) => {
          if (!version || !selected?.gradeLevel) return;
          await api.createTopic.mutateAsync({
            curriculum_version_id: version.curriculum_version_id,
            code,
            name,
            subject_id: selected.subjectId,
            grade_level: selected.gradeLevel,
            ...(selected.kind === 'topic' && selected.topic ? { parent_topic_id: selected.topic.topic_id } : {}),
          });
          refresh();
        }}
      />
      <CreateCurriculumDialog
        open={creating === 'curriculum'}
        crossTenant={access.crossTenant}
        onClose={() => setCreating(null)}
        onCreate={async (code, name, owner) => {
          const o = ownerOf(owner);
          const created = await api.createCurriculum.mutateAsync({ body: { code, name: { vi: name }, ...(o.body.tenant_id ? { tenant_id: o.body.tenant_id } : {}) }, headers: o.headers });
          await qc.invalidateQueries({ queryKey: ['content-studio', 'curricula', scopeKey] });
          setCurriculumId(created.curriculum_id);
        }}
      />
    </section>
  );
}

function CreateTopicDialog({ open, onClose, onCreate }: { open: boolean; onClose: () => void; onCreate: (code: string, name: string) => Promise<unknown> }) {
  const { t } = useTranslation();
  const [form] = Form.useForm<{ code: string; name: string }>();
  const [error, setError] = useState<unknown>(null);
  return (
    <Modal
      open={open}
      title={t('content_studio.curriculum.addTopic')}
      okText={t('content_studio.create')}
      cancelText={t('content_studio.cancel')}
      destroyOnHidden
      onCancel={onClose}
      onOk={async () => {
        const v = await form.validateFields().catch(() => null);
        if (!v) return;
        try {
          await onCreate(v.code.trim(), v.name.trim());
          onClose();
        } catch (e) {
          setError(e);
        }
      }}
    >
      <Form form={form} layout="vertical" preserve={false}>
        <Form.Item name="code" label={t('content_studio.curriculum.topicCode')} rules={[{ required: true, message: t('content_studio.validation.required') }, { pattern: /^[A-Z0-9][A-Z0-9_.]{0,63}$/, message: t('content_studio.validation.pattern') }]}>
          <Input maxLength={LIMITS.topicCode} showCount />
        </Form.Item>
        <Form.Item name="name" label={t('content_studio.curriculum.topicName')} rules={[{ required: true, whitespace: true, message: t('content_studio.validation.required') }]}>
          <Input maxLength={LIMITS.topicName} showCount />
        </Form.Item>
      </Form>
      {error ? <ProblemMessage error={error} /> : null}
    </Modal>
  );
}

function CreateCurriculumDialog({ open, crossTenant, onClose, onCreate }: { open: boolean; crossTenant: boolean; onClose: () => void; onCreate: (code: string, name: string, owner?: string) => Promise<unknown> }) {
  const { t } = useTranslation();
  const [form] = Form.useForm<{ code: string; name: string; [OWNER_FIELD]?: string }>();
  const [error, setError] = useState<unknown>(null);
  return (
    <Modal
      open={open}
      title={t('content_studio.curriculum.createOwn')}
      okText={t('content_studio.create')}
      cancelText={t('content_studio.cancel')}
      destroyOnHidden
      onCancel={onClose}
      onOk={async () => {
        const v = await form.validateFields().catch(() => null);
        if (!v) return;
        try {
          await onCreate(v.code.trim(), v.name.trim(), v[OWNER_FIELD]);
          onClose();
        } catch (e) {
          setError(e);
        }
      }}
    >
      <Form form={form} layout="vertical" preserve={false}>
        {crossTenant && <OwnerScopeField />}
        <Form.Item name="code" label={t('content_studio.curriculum.code')} rules={[{ required: true, message: t('content_studio.validation.required') }, { pattern: /^[A-Z][A-Z0-9_]{1,63}$/, message: t('content_studio.validation.pattern') }]}>
          <Input maxLength={LIMITS.topicCode} showCount />
        </Form.Item>
        <Form.Item name="name" label={t('content_studio.curriculum.name')} rules={[{ required: true, whitespace: true, message: t('content_studio.validation.required') }]}>
          <Input maxLength={LIMITS.topicName} showCount />
        </Form.Item>
      </Form>
      {error ? <ProblemMessage error={error} /> : null}
    </Modal>
  );
}
