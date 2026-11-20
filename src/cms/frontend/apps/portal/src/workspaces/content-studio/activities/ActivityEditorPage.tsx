// ActivityEditorPage (F058 US1, US2, US5; F058-AC-02…AC-14, AC-26…AC-30, ADMIN_PORTAL §1.4, §3.13): one learning
// activity on ContentEditorShell (F053) — metadata, bilingual description / instructions, the config editor of its
// type from the editor registry, schema check of the config before saving, autosave of the draft (full
// ActivityDefinitionInput + If-Match; 412/409 stops autosave and offers to reload), workflow of the version (submit /
// approve / reject; publish through PublishDialog), "Tạo bản nháp từ v{n}" for a published or not editable version,
// publish locked while the type is `preview` in the registry, release tabs (versions, releases, usages, preview "như học
// sinh thấy"), system re-evaluation of a published version. Builders need ≥ 1024 px; smaller screens read only.
// The page orchestrates only: rules and grading live in module learning (AGENTS R1, R5).
import { BilingualField, ContentEditorShell, toLangMap, useAutosave, type EditorBlocker, type I18nTextValue, type WorkflowAction } from '@eduworld/ops-ui/content';
import { Alert, Button, Card, Form, InputNumber, Result, Spin, Tag } from 'antd';
import { useCallback, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { ActionGate } from '../../../permissions/ActionGate';
import { registerUnsavedChanges } from '../../../session/unsavedChangesGuard';
import { useMe } from '../../../shared/api/hooks/useMe';
import { ApiError } from '../../../shared/api/middleware/problem';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { ReasonDialog } from '../../../shell/ReasonDialog';
import { LargeScreenRequired } from '../../../shell/StatusPages';
import { useMediaQuery } from '../../../shell/useMediaQuery';
import { DocMetaFields } from '../builders/DocMetaFields';
import { useWorkflow } from '../questions/useWorkflow';
import { ContentReleaseTabs } from '../release/ContentReleaseTabs';
import { PublishDialog } from '../release/PublishDialog';
import { useRelease } from '../release/useRelease';
import { ReviewFieldComments } from '../review/ReviewFieldComments';
import { useContentAccess } from '../shared/access';
import { AssignedBadge } from '../shared/AssignedBadge';
import { CONTENT_AUTOSAVE_DEBOUNCE_MS, CONTENT_AUTOSAVE_INTERVAL_MS, EDITOR_MIN_WIDTH_QUERY, LIMITS } from '../shared/config';
import { CONTENT_TYPES } from '../shared/contentTypes';
import { OWNER_FIELD, ownerOf, OwnerScopeField } from '../shared/OwnerScopeField';
import { RegradeEntryButton } from '../regrade/RegradeEntryButton';
import { emptyActivity, fromDefinition, metaErrors, toInput, validateConfig, type ActivityDefinition, type ActivityState, type ActivityType } from './activityModel';
import { activityEditor } from './editorRegistry';
import { useActivityApi, useActivityQuery, useActivityTypes } from './useActivity';
import './editors';

const EDITABLE = new Set(['draft', 'rejected']);
const isConflict = (e: unknown) => e instanceof ApiError && (e.status === 412 || e.code === 'CONTENT_VERSION_CONFLICT' || e.code === 'VERSION_MISMATCH');
/** LearningText.maxLength (description, instructions). */
const TEXT_MAX = 20000;
const ACTIONS = CONTENT_TYPES.activity.actions;

export function ActivityEditorPage({ id, type }: { id?: string; type?: ActivityType }) {
  const { t } = useTranslation();
  const q = useActivityQuery(id);
  const [nonce, setNonce] = useState(0);
  if (id && q.isPending) return <Spin aria-label={t('content_studio.loading')} />;
  if (id && q.error) return q.error instanceof ApiError && q.error.status === 404 ? <Result status="404" title={t('content_studio.notFound')} /> : <ProblemMessage error={q.error} />;
  const server = q.data?.data;
  const editorType = (server?.activity_type ?? type) as ActivityType | undefined;
  if (!editorType || !activityEditor(editorType)) return <Result status="warning" title={t('content_studio.activity.noEditor', { type: editorType ?? '—' })} />;
  return (
    <ActivityForm
      key={`${id ?? 'new'}:${server?.version ?? 0}:${q.data?.etag ?? ''}:${nonce}`}
      id={id}
      type={editorType}
      server={server}
      etag={q.data?.etag ?? null}
      reload={async () => {
        await q.refetch();
        setNonce((n) => n + 1);
      }}
    />
  );
}

function ActivityForm({ id, type, server, etag, reload }: { id?: string; type: ActivityType; server?: ActivityDefinition; etag: string | null; reload: () => Promise<void> }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const access = useContentAccess();
  const wide = useMediaQuery(EDITOR_MIN_WIDTH_QUERY);
  const me = useMe();
  const types = useActivityTypes();
  const api = useActivityApi();
  const { transition } = useWorkflow();
  const { newDraft } = useRelease();
  const def = activityEditor(type)!;
  const typeInfo = types.data?.data.find((x) => x.activity_type === type);
  const initial = useMemo<ActivityState>(() => (server ? fromDefinition(server) : emptyActivity(type, def.empty())), [server, type, def]);
  const [state, setState] = useState<ActivityState>(initial);
  const [serverErrors, setServerErrors] = useState<Record<string, string>>({});
  const [actionError, setActionError] = useState<unknown>(null);
  const [notEditable, setNotEditable] = useState(false);
  const [busy, setBusy] = useState<WorkflowAction | null>(null);
  const [dialog, setDialog] = useState<'reject' | 'publish' | null>(null);
  const [mediaStatus, setMediaStatus] = useState<Record<string, string>>({});
  const [flags, setFlags] = useState<Record<string, boolean>>({});
  const [ownerForm] = Form.useForm<{ [OWNER_FIELD]?: string }>();
  const status = server?.status ?? 'draft';
  const assigned = !!server?.assigned;
  const readOnly = !wide || assigned || notEditable || !access.activity.write || (!!server && !EDITABLE.has(status));
  const preview = typeInfo?.status === 'preview';

  const schemaErrors = useMemo(() => {
    const out: Record<string, string> = {};
    for (const e of validateConfig(type, state.config)) out[e.pointer] = t(`content_studio.validation.${['maxLength', 'minLength', 'maxItems', 'minItems', 'required', 'pattern', 'enum', 'type', 'minimum', 'maximum'].includes(e.keyword) ? e.keyword : 'invalid'}`, { limit: e.params.limit ?? '' });
    return out;
  }, [type, state.config, t]);
  const errors = useMemo(() => ({ ...schemaErrors, ...serverErrors }), [schemaErrors, serverErrors]);
  const onMediaStatus = useCallback((assetId: string, st: string) => setMediaStatus((m) => (m[assetId] === st ? m : { ...m, [assetId]: st })), []);
  const local = useMemo<EditorBlocker[]>(
    () => [
      ...metaErrors(state).map((f) => ({ field: f, message: t(`content_studio.activity.meta.${f.slice(1)}`) })),
      ...(def.blockers?.(state.config, { t, mediaStatus, flags }) ?? []),
      ...Object.entries(schemaErrors).map(([field, m]) => ({ field, message: `${field}: ${m}` })),
    ],
    [state, def, t, mediaStatus, flags, schemaErrors],
  );

  const capture = (e: unknown) => {
    if (!(e instanceof ApiError)) return;
    if (e.code === 'CONTENT_NOT_EDITABLE') setNotEditable(true);
    const out: Record<string, string> = {};
    for (const [f, fe] of Object.entries(e.fieldErrors)) out[f.startsWith('/') ? f : `/${f}`] = fe.message ?? t(`error.field.${fe.code}`, { defaultValue: t(e.i18nKey) });
    setServerErrors(out);
  };

  const autosave = useAutosave<ActivityState>(state, !!id && !readOnly && local.length === 0, {
    intervalMs: CONTENT_AUTOSAVE_INTERVAL_MS,
    debounceMs: CONTENT_AUTOSAVE_DEBOUNCE_MS,
    initialEtag: etag,
    isConflict,
    save: async (v, currentEtag) => {
      try {
        const r = await api.saveDraft(id!, toInput(v), currentEtag);
        setServerErrors({});
        return { etag: r.etag };
      } catch (e) {
        capture(e);
        throw e;
      }
    },
  });

  const dirtyRef = useRef(false);
  dirtyRef.current = autosave.dirty || (!id && JSON.stringify(state) !== JSON.stringify(initial));
  const registerGuard = useCallback((isDirty: () => boolean) => registerUnsavedChanges(`activity:${id ?? 'new'}`, isDirty), [id]);
  const create = useIdempotentMutation(async (v: { body: Record<string, unknown>; headers: Record<string, string> }, intent) => api.create(v.body as never, { ...intent, ...v.headers }));

  const fail = (e: unknown) => {
    capture(e);
    setActionError(e);
  };

  const onAction = async (a: WorkflowAction) => {
    setActionError(null);
    if (a === 'reject' || a === 'publish') return setDialog(a);
    setBusy(a);
    try {
      if (!id) {
        let owner = ownerOf(null);
        if (access.crossTenant) {
          const v = await ownerForm.validateFields().catch(() => null);
          if (!v) return;
          owner = ownerOf(v[OWNER_FIELD]);
        }
        const r = await create.mutateAsync({ body: { ...toInput(state), ...owner.body }, headers: owner.headers });
        api.remember(r.data.activity_id, r);
        navigate(`../${r.data.activity_id}`, { replace: true });
        return;
      }
      if (a === 'save') return void (await autosave.saveNow());
      if (a === 'new_draft') {
        await newDraft.mutateAsync({ contentType: 'activity', contentId: id, fromVersion: server!.version });
        await reload();
        return;
      }
      if (a === 'submit_review') {
        await autosave.saveNow();
        const now = autosave.current();
        if (now.state === 'error' || now.state === 'conflict' || now.dirty) return;
      }
      await transition.mutateAsync({ contentType: 'activity', contentId: id, version: server!.version, action: a as 'submit_review' | 'approve', etag: autosave.current().etag });
      await reload();
    } catch (e) {
      fail(e);
    } finally {
      setBusy(null);
    }
  };

  const set = (patch: Partial<ActivityState>) => setState((s) => ({ ...s, ...patch }));
  const errorBlockers = Object.entries(serverErrors).map(([field, msg]) => ({ field, message: `${field}: ${msg}` }));
  const Editor = def.Editor;
  const err = (p: string) => errors[p];
  const published = status === 'published' || (server?.current_published_version ?? 0) >= (server?.version ?? 1);
  return (
    <>
      {!wide && <LargeScreenRequired />}
      {actionError ? <ProblemMessage error={actionError} /> : null}
      {preview && (
        <Alert type="warning" showIcon data-testid="type-preview" message={t('content_studio.activity.previewType', { version: typeInfo?.min_client_version ?? '—' })} />
      )}
      {(notEditable || (server && status === 'published')) && id && server && (
        <Alert
          type="warning"
          showIcon
          message={notEditable ? t('error.CONTENT_NOT_EDITABLE') : t('content_studio.activity.publishedReadOnly', { version: server.version })}
          action={
            <ActionGate actionKey={ACTIONS.edit!}>
              <Button size="small" onClick={() => void onAction('new_draft')}>
                {t('content_studio.release.newDraftFrom', { version: server.version })}
              </Button>
            </ActionGate>
          }
        />
      )}
      <ContentEditorShell
        title={id ? t('content_studio.activity.editTitle', { type: t(def.labelKey) }) : t('content_studio.activity.createTitle', { type: t(def.labelKey) })}
        badges={
          <>
            {assigned ? <AssignedBadge /> : null}
            <Tag>{t(def.labelKey)}</Tag>
          </>
        }
        dirty={dirtyRef.current}
        registerDirtyGuard={registerGuard}
        conflict={autosave.state === 'conflict' ? { onReload: () => void reload() } : null}
        statusBar={{
          status,
          version: server?.version ?? 1,
          saveState: autosave.state,
          lastSavedAt: autosave.lastSavedAt,
          can: { write: access.activity.write, review: access.activity.review, publish: access.activity.publish && !preview },
          isAuthor: !!server?.author_id && server.author_id === me.data?.user_id,
          readOnly: !wide || assigned,
          blockers: [...local, ...errorBlockers],
          busy,
          onAction: (a) => void onAction(a),
          wrapAction: (a, b) => {
            if (!id && a !== 'save') return null;
            const key = a === 'publish' ? ACTIONS.publish : a === 'approve' || a === 'reject' || a === 'submit_review' ? ACTIONS.workflow : ACTIONS.edit!;
            return (
              <ActionGate actionKey={key} mode="disable">
                {b}
              </ActionGate>
            );
          },
        }}
        aside={id && server && published ? <RegradeEntryButton scope="activity_version" versionId={server.activity_version_id} /> : undefined}
      >
        {!id && access.crossTenant && (
          <Card size="small">
            <Form form={ownerForm} layout="vertical">
              <OwnerScopeField />
            </Form>
          </Card>
        )}
        {id && server && EDITABLE.has(status) && <ReviewFieldComments item={{ contentType: 'activity', contentId: id }} version={server.version} />}
        <Card size="small" title={t('content_studio.activity.metaTitle')}>
          <DocMetaFields
            value={state}
            onChange={(v) => set({ title: toLangMap(v.title as I18nTextValue, v.content_language), content_language: v.content_language, subject_id: v.subject_id, grade_levels: v.grade_levels, topic_ids: v.topic_ids, objective_ids: v.objective_ids })}
            readOnly={readOnly}
            languageLocked={!!id}
            singleTopic
            errors={errors}
          />
          <Form layout="vertical" component="div" disabled={readOnly}>
            <Form.Item label={t('content_studio.activity.difficulty')} validateStatus={err('/difficulty') ? 'error' : undefined} help={err('/difficulty')}>
              <InputNumber aria-label={t('content_studio.activity.difficulty')} min={1} max={5} value={state.difficulty} onChange={(v) => set({ difficulty: v ?? undefined })} />
            </Form.Item>
            <BilingualField id="description" label={t('content_studio.activity.description')} value={state.description} onChange={(v) => set({ description: toLangMap(v as I18nTextValue, state.content_language) })} contentLanguage={state.content_language} maxLength={TEXT_MAX} readOnly={readOnly} error={err('/description')} />
            <BilingualField id="instructions" label={t('content_studio.activity.instructions')} value={state.instructions} onChange={(v) => set({ instructions: toLangMap(v as I18nTextValue, state.content_language) })} contentLanguage={state.content_language} maxLength={TEXT_MAX} readOnly={readOnly} error={err('/instructions')} />
            <Form.Item label={t('content_studio.activity.timeLimit')} extra={t('content_studio.activity.timeLimitHelp')}>
              <InputNumber aria-label={t('content_studio.activity.timeLimit')} min={1} value={state.time_limit_seconds} onChange={(v) => set({ time_limit_seconds: v ?? undefined })} />
            </Form.Item>
          </Form>
        </Card>
        <Editor
          config={state.config}
          onChange={(c) => set({ config: c })}
          readOnly={readOnly}
          errors={errors}
          typeInfo={typeInfo}
          gradeLevels={state.grade_levels}
          subjectId={state.subject_id}
          onMediaStatus={onMediaStatus}
          flags={flags}
          onFlag={(name, value) => setFlags((f) => ({ ...f, [name]: value }))}
        />
      </ContentEditorShell>
      {id && server && (
        <Card>
          <ContentReleaseTabs item={{ contentType: 'activity', contentId: id }} version={server.version} assigned={assigned} contentLanguage={server.content_language === 'en' ? 'en' : 'vi'} onDraftCreated={() => void reload()} />
        </Card>
      )}
      <ReasonDialog
        open={dialog === 'reject'}
        title={t('content_studio.question.rejectTitle')}
        maxLength={LIMITS.transitionNote}
        confirmLoading={transition.isPending}
        onCancel={() => setDialog(null)}
        onSubmit={(note) => {
          setDialog(null);
          setBusy('reject');
          transition
            .mutateAsync({ contentType: 'activity', contentId: id!, version: server!.version, action: 'reject', note, etag: autosave.current().etag })
            .then(reload, fail)
            .finally(() => setBusy(null));
        }}
      />
      {id && server && (
        <PublishDialog
          item={{ contentType: 'activity', contentId: id }}
          version={dialog === 'publish' && !preview ? server.version : null}
          notice={server.current_published_version ? t('content_studio.activity.publishPinned') : undefined}
          onClose={() => {
            setDialog(null);
            void reload();
          }}
        />
      )}
    </>
  );
}
