// ActivityEditorPage (F058 US1, US2, US5; F058-AC-02…AC-14, AC-26…AC-30, ADMIN_PORTAL §1.4, §3.13): one learning
// activity on ContentEditorShell (F053) — metadata, bilingual description / instructions, the config editor of its
// type from the editor registry, schema check of the config before saving, autosave of the draft (full
// ActivityDefinitionInput + If-Match; 412/409 stops autosave and offers to reload), workflow of the version (submit /
// approve / reject; publish through PublishDialog), "Tạo bản nháp từ v{n}" for a published or not editable version,
// publish locked while the type is `preview` in the registry, release tabs (versions, releases, usages, preview "như học
// sinh thấy"), system re-evaluation of a published version. Builders need ≥ 1024 px; smaller screens read only.
// The page orchestrates only: rules and grading live in module learning (AGENTS R1, R5).
import {
  BilingualField,
  buildMathAssets,
  collectMath,
  ContentEditorShell,
  MathInsertButton,
  mathId,
  toLangMap,
  useAutosave,
  type EditorBlocker,
  type I18nTextValue,
  type MathBuildResult,
  type WorkflowAction,
} from '@eduworld/ops-ui/content';
import { Alert, Button, Card, Form, Input, InputNumber, Result, Spin, Tag } from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
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
import { MediaPicker } from '../media/MediaPicker';
import { useWorkflow } from '../questions/useWorkflow';
import { ContentReleaseTabs } from '../release/ContentReleaseTabs';
import { PublishDialog } from '../release/PublishDialog';
import { useRelease } from '../release/useRelease';
import { ReviewFieldComments } from '../review/ReviewFieldComments';
import { useContentAccess } from '../shared/access';
import { AssignedBadge } from '../shared/AssignedBadge';
import { CONTENT_AUTOSAVE_DEBOUNCE_MS, CONTENT_AUTOSAVE_INTERVAL_MS, EDITOR_MIN_WIDTH_QUERY, LIMITS } from '../shared/config';
import { CONTENT_TYPES } from '../shared/contentTypes';
import { needsSpokenStem } from '../shared/gates';
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
/** math_assets.maxItems of ActivityDefinitionInput; audio_exempt_reason.maxLength. */
const MATH_MAX = 200;
const EXEMPT_MAX = 255;
const NO_MATH: MathBuildResult = { assets: [], assetPaths: [], errors: [] };

/** Fields that may hold formulas (F058-AC-08): description, instructions and the texts of the config. */
const mathPayload = (s: ActivityState) => ({ description: s.description, instructions: s.instructions, config: s.config });

/** Dotted path of a formula → JSON Pointer of its field (`instructions.vi` → `/instructions`, `config.content.segments.0.title.vi` → `/config/content/segments/0/title`). */
function mathPointer(path: string): string {
  const parts = path.split('.');
  if (parts[0] === 'description' || parts[0] === 'instructions') return `/${parts[0]}`;
  const tail = parts.at(-1) === 'vi' || parts.at(-1) === 'en' ? parts.slice(0, -1) : parts;
  return `/${tail.join('/')}`;
}

/** Renders the formulas of a state (MathJax in the browser, ADR-0055) into math_assets with their math_id. */
async function renderMathAssets(s: ActivityState): Promise<{ built: MathBuildResult; assets: ActivityState['math_assets'] }> {
  const built = await buildMathAssets(collectMath(mathPayload(s), s.content_language));
  const assets = await Promise.all(built.assets.slice(0, MATH_MAX).map(async (a) => ({ math_id: await mathId(a.latex, false), latex: a.latex, svg: a.svg, speech_text: a.speech_text })));
  return { built, assets };
}

/** Appends a formula to the text of one language (F058-AC-08, MathInsertButton). */
const appendTo = (v: ActivityState['description'], lang: 'vi' | 'en', formula: string) => ({ ...v, [lang]: `${v[lang] ?? ''} ${formula}`.trim() });

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
  // F058-AC-08: formulas are rendered as the author types; a formula error is shown on its field and blocks saving.
  const [math, setMath] = useState<MathBuildResult>(NO_MATH);
  const lastBuilt = useRef<MathBuildResult>(NO_MATH);
  useEffect(() => {
    let live = true;
    void buildMathAssets(collectMath(mathPayload(state), state.content_language)).then((r) => live && setMath(r));
    return () => {
      live = false;
    };
  }, [state]);
  const mathErrors = useMemo(() => {
    const out: Record<string, string> = {};
    for (const e of math.errors) out[mathPointer(e.path)] ??= t('content_studio.gate.math', { latex: e.latex.slice(0, 40) });
    return out;
  }, [math.errors, t]);
  const errors = useMemo(() => ({ ...schemaErrors, ...mathErrors, ...serverErrors }), [schemaErrors, mathErrors, serverErrors]);
  const onMediaStatus = useCallback((assetId: string, st: string) => setMediaStatus((m) => (m[assetId] === st ? m : { ...m, [assetId]: st })), []);
  const local = useMemo<EditorBlocker[]>(
    () => [
      ...metaErrors(state).map((f) => ({ field: f, message: t(`content_studio.activity.meta.${f.slice(1)}`) })),
      ...(def.blockers?.(state.config, { t, mediaStatus, flags }) ?? []),
      ...Object.entries(schemaErrors).map(([field, m]) => ({ field, message: `${field}: ${m}` })),
      ...Object.entries(mathErrors).map(([field, message]) => ({ field, message })),
    ],
    [state, def, t, mediaStatus, flags, schemaErrors, mathErrors],
  );
  // F058-AC-09 (CONTENT_MODEL §8.1): grades 1–3 need the voice-over of the instructions or an exemption. It blocks
  // Gửi duyệt only: a draft can be saved before the recording exists.
  const spoken = needsSpokenStem(state.grade_levels);
  const submitGates = useMemo<EditorBlocker[]>(() => {
    if (!spoken) return [];
    const ref = state.instructions_audio_ref;
    if (!ref && !state.audio_exempt_reason?.trim()) return [{ field: '/instructions_audio_ref', message: t('content_studio.activity.audioRequired') }];
    if (ref && mediaStatus[ref] && mediaStatus[ref] !== 'ready') return [{ field: '/instructions_audio_ref', message: t('error.MEDIA_NOT_READY') }];
    return [];
  }, [spoken, state.instructions_audio_ref, state.audio_exempt_reason, mediaStatus, t]);

  const capture = (e: unknown) => {
    if (!(e instanceof ApiError)) return;
    if (e.code === 'CONTENT_NOT_EDITABLE') setNotEditable(true);
    const out: Record<string, string> = {};
    for (const [f, fe] of Object.entries(e.fieldErrors)) {
      const m = e.code === 'MATH_RENDER_FAILED' ? /^\/?math_assets\/(\d+)/.exec(f) : null;
      if (m) {
        // F058-AC-08: the server refused a formula → red on the fields holding it
        for (const p of lastBuilt.current.assetPaths[Number(m[1])] ?? []) out[mathPointer(p)] = t('content_studio.problem.MATH_RENDER_FAILED');
        continue;
      }
      out[f.startsWith('/') ? f : `/${f}`] = fe.message ?? t(`error.field.${fe.code}`, { defaultValue: t(e.i18nKey) });
    }
    setServerErrors(out);
  };

  const autosave = useAutosave<ActivityState>(state, !!id && !readOnly && local.length === 0, {
    intervalMs: CONTENT_AUTOSAVE_INTERVAL_MS,
    debounceMs: CONTENT_AUTOSAVE_DEBOUNCE_MS,
    initialEtag: etag,
    isConflict,
    save: async (v, currentEtag) => {
      try {
        const { built, assets } = await renderMathAssets(v);
        lastBuilt.current = built;
        const r = await api.saveDraft(id!, toInput({ ...v, math_assets: assets }), currentEtag);
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
        const { built, assets } = await renderMathAssets(state);
        lastBuilt.current = built;
        const r = await create.mutateAsync({ body: { ...toInput({ ...state, math_assets: assets }), ...owner.body }, headers: owner.headers });
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
          blockers: [...local, ...submitGates, ...errorBlockers],
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
            <Form.Item>
              <MathInsertButton language={state.content_language} disabled={readOnly} onInsert={(m) => set({ description: appendTo(state.description, state.content_language, m) })} />
            </Form.Item>
            <BilingualField id="instructions" label={t('content_studio.activity.instructions')} value={state.instructions} onChange={(v) => set({ instructions: toLangMap(v as I18nTextValue, state.content_language) })} contentLanguage={state.content_language} maxLength={TEXT_MAX} readOnly={readOnly} error={err('/instructions')} />
            <Form.Item>
              <MathInsertButton language={state.content_language} disabled={readOnly} onInsert={(m) => set({ instructions: appendTo(state.instructions, state.content_language, m) })} />
            </Form.Item>
            {spoken && (
              <>
                <Form.Item label={t('content_studio.activity.instructionsAudio')} extra={t('content_studio.activity.instructionsAudioHelp')} validateStatus={err('/instructions_audio_ref') ? 'error' : undefined} help={err('/instructions_audio_ref')}>
                  <MediaPicker id="instructions_audio_ref" kind="audio" value={state.instructions_audio_ref} disabled={readOnly} onChange={(v) => set({ instructions_audio_ref: v })} onStatus={onMediaStatus} />
                </Form.Item>
                <Form.Item label={t('content_studio.activity.audioExempt')} extra={t('content_studio.activity.audioExemptHelp')}>
                  <Input id="audio_exempt_reason" aria-label={t('content_studio.activity.audioExempt')} maxLength={EXEMPT_MAX} showCount value={state.audio_exempt_reason ?? ''} onChange={(e) => set({ audio_exempt_reason: e.target.value })} />
                </Form.Item>
              </>
            )}
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
