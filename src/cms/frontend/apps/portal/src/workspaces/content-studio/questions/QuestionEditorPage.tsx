// QuestionEditorPage (F053 US1–US3: AC-01…AC-25 for questions; ADMIN_PORTAL §1.4, §3.2): loads a question version
// (or starts a new one), edits it in ContentEditorShell + QuestionEditor + metadata, autosaves the draft with If-Match,
// renders formulas to math_assets before every save, gates Gửi duyệt / Xuất bản (answer key, schema, formulas,
// spoken stem g1_3, original-language texts, media pipeline, server 422 lists), runs the workflow of the item, and
// is read only for published / in-review / assigned versions, without `learning_content.write`, or below 1024 px.
// Business rules stay on the server; this page only orchestrates (AGENTS R1).
import {
  answerIssues,
  buildMathAssets,
  collectMath,
  ContentEditorShell,
  newQuestionDraft,
  QuestionEditor,
  TranslationBadge,
  type EditorBlocker,
  type MathBuildResult,
  type MediaSlotArgs,
  type QuestionDraft,
  type WorkflowAction,
} from '@eduworld/ops-ui/content';
import { Alert, Card, Flex, Form, Input, Modal, Result, Spin, Tabs } from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { useAutosave } from '@eduworld/ops-ui/content';
import { registerUnsavedChanges } from '../../../session/unsavedChangesGuard';
import { useMe } from '../../../shared/api/hooks/useMe';
import { ApiError } from '../../../shared/api/middleware/problem';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { RegradeEntryButton } from '../regrade/RegradeEntryButton';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { ReasonDialog } from '../../../shell/ReasonDialog';
import { LargeScreenRequired } from '../../../shell/StatusPages';
import { useMediaQuery } from '../../../shell/useMediaQuery';
import { MediaPicker } from '../media/MediaPicker';
import { ReviewFieldComments } from '../review/ReviewFieldComments';
import { gateQuestionAction, useContentAccess } from '../shared/access';
import { AssignedBadge } from '../shared/AssignedBadge';
import { ASSETS_PER_VERSION_MAX, CONTENT_AUTOSAVE_DEBOUNCE_MS, CONTENT_AUTOSAVE_INTERVAL_MS, EDITOR_MIN_WIDTH_QUERY, LIMITS } from '../shared/config';
import { audioGate, mediaGate, mediaRefs, originalMissing, serverGates, translationPartial, type Gate } from '../shared/gates';
import { OWNER_FIELD, ownerOf, OwnerScopeField } from '../shared/OwnerScopeField';
import { schemaErrorMessage, validateQuestionDraft } from '../shared/schemaValidator';
import { QuestionMetaPanel } from './QuestionMetaPanel';
import { StudentViewPreview } from './StudentViewPreview';
import { fromAuthoring, toAuthoringBody, useCmsQuestion, useQuestionApi, type QuestionAuthoring, type QuestionMeta } from './useQuestion';
import { useWorkflow } from './useWorkflow';

interface EditorState {
  draft: QuestionDraft;
  meta: QuestionMeta;
}

const EDITABLE = new Set(['draft', 'rejected']);
const isConflict = (e: unknown) => e instanceof ApiError && (e.status === 412 || e.code === 'CONTENT_VERSION_CONFLICT' || e.code === 'VERSION_MISMATCH');
/** `stem.blocks.0.text.vi` (formula path) → field id `stem.blocks.0.text`. */
const fieldOf = (path: string) => path.replace(/\.(vi|en)$/, '');

/** Scrolls to and focuses the field of a blocker (F053-AC-23). */
export function jumpToField(field: string): void {
  let f = field;
  for (;;) {
    const el = document.getElementById(`${f}.vi`) ?? document.getElementById(f) ?? Array.from(document.querySelectorAll<HTMLElement>('[data-field]')).find((x) => x.dataset.field === f);
    if (el) {
      el.scrollIntoView?.({ block: 'center' });
      const target = el.matches?.('input,textarea') ? el : el.querySelector?.<HTMLElement>('input,textarea,[tabindex]');
      (target ?? el).focus?.();
      return;
    }
    if (!f.includes('.')) return;
    f = f.slice(0, f.lastIndexOf('.'));
  }
}

export function QuestionEditorPage({ questionId }: { questionId?: string }) {
  const { t } = useTranslation();
  const q = useCmsQuestion(questionId);
  const [nonce, setNonce] = useState(0);
  if (questionId && q.isPending) return <Spin aria-label={t('content_studio.loading')} />;
  if (questionId && q.error) {
    return q.error instanceof ApiError && q.error.status === 404 ? <Result status="404" title={t('content_studio.question.notFound')} /> : <ProblemMessage error={q.error} />;
  }
  const loaded = q.data;
  return (
    <QuestionEditorForm
      key={`${questionId ?? 'new'}:${loaded?.data.version ?? 0}:${loaded?.etag ?? ''}:${nonce}`}
      questionId={questionId}
      server={loaded?.data}
      etag={loaded?.etag ?? null}
      reload={async () => {
        await q.refetch();
        setNonce((n) => n + 1);
      }}
    />
  );
}

function QuestionEditorForm({ questionId, server, etag, reload }: { questionId?: string; server?: QuestionAuthoring; etag: string | null; reload: () => Promise<void> }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const access = useContentAccess();
  const wide = useMediaQuery(EDITOR_MIN_WIDTH_QUERY);
  const me = useMe();
  const qapi = useQuestionApi();
  const { transition, publish } = useWorkflow();
  const initial = useMemo<EditorState>(() => (server ? fromAuthoring(server) : { draft: newQuestionDraft('single_choice'), meta: { content_language: 'vi' } }), [server]);
  const [state, setState] = useState<EditorState>(initial);
  const [math, setMath] = useState<MathBuildResult>({ assets: [], assetPaths: [], errors: [] });
  const [server422, setServer422] = useState<Gate[]>([]);
  const [actionError, setActionError] = useState<unknown>(null);
  const [mediaStatus, setMediaStatus] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState<WorkflowAction | null>(null);
  const [dialog, setDialog] = useState<'reject' | 'publish' | null>(null);
  const [ownerForm] = Form.useForm<{ [OWNER_FIELD]?: string }>();
  const lang = state.meta.content_language;
  const status = server?.status ?? 'draft';
  const assigned = !!server?.assigned;
  const readOnly = !wide || assigned || !access.question.write || (!!server && !EDITABLE.has(status));
  const outdated = useMemo(() => new Set(((server as { outdated_fields?: string[] } | undefined)?.outdated_fields ?? []).map((p) => p.replace(/^\//, '').split('/').join('.'))), [server]);

  // formulas → SVG + speech in the browser (cached per formula), recomputed when the content changes
  useEffect(() => {
    let live = true;
    const { draft } = state;
    void buildMathAssets(collectMath({ stem: draft.stem, render_payload: draft.render_payload, explanation: draft.explanation, hints: draft.hints }, lang)).then((r) => live && setMath(r));
    return () => {
      live = false;
    };
  }, [state, lang]);

  const issues = useMemo(() => answerIssues(state.draft), [state.draft]);
  const schemaErrors = useMemo(() => validateQuestionDraft(state.draft), [state.draft]);
  const blockingLocal = issues.length + schemaErrors.length + math.errors.length > 0;

  const errors = useMemo(() => {
    const out: Record<string, string> = {};
    for (const i of issues) out[i.field] ??= t(`content_studio.issue.${i.code}`, { value: i.value ?? '' });
    for (const e of schemaErrors) out[e.field] ??= schemaErrorMessage(e, t);
    for (const e of math.errors) out[fieldOf(e.path)] ??= t(`ops-ui:math.error.${e.error.code}`, { detail: e.error.detail ?? '', max: e.error.detail ?? '' });
    for (const g of server422) if (g.field) out[g.field] ??= t(g.key, g.params);
    return out;
  }, [issues, schemaErrors, math.errors, server422, t]);

  const autosave = useAutosave<EditorState>(state, !!questionId && !readOnly && !blockingLocal, {
    intervalMs: CONTENT_AUTOSAVE_INTERVAL_MS,
    debounceMs: CONTENT_AUTOSAVE_DEBOUNCE_MS,
    initialEtag: etag,
    isConflict,
    save: async (v, currentEtag) => {
      const built = await buildMathAssets(collectMath({ stem: v.draft.stem, render_payload: v.draft.render_payload, explanation: v.draft.explanation, hints: v.draft.hints }, v.meta.content_language));
      const body = toAuthoringBody(v.draft, v.meta, built.assets, mediaRefs(v.draft));
      try {
        const r = await qapi.saveDraft(questionId!, server!.version, body, currentEtag);
        setServer422([]);
        return { etag: r.etag };
      } catch (e) {
        if (e instanceof ApiError) setServer422(e.code === 'MATH_RENDER_FAILED' ? mathGates(e, built) : serverGates(e));
        throw e;
      }
    },
  });

  const dirtyRef = useRef(false);
  dirtyRef.current = autosave.dirty || (!questionId && JSON.stringify(state) !== JSON.stringify(initial));
  const registerGuard = useCallback((isDirty: () => boolean) => registerUnsavedChanges(`question:${questionId ?? 'new'}`, isDirty), [questionId]);

  const change = (next: EditorState) => {
    setState(next);
    if (server422.length) setServer422([]);
  };

  const gateMsg = (g: Gate): EditorBlocker => ({ field: g.field, message: t(g.key, g.params) });
  const blockers: EditorBlocker[] = [
    ...issues.map((i) => ({ field: i.field, message: t(`content_studio.issue.${i.code}`, { value: i.value ?? '' }) })),
    ...schemaErrors.map((e) => ({ field: e.field, message: `${t('content_studio.validation.field', { field: e.field })} ${schemaErrorMessage(e, t)}` })),
    ...math.errors.map((e) => ({ field: fieldOf(e.path), message: t('content_studio.gate.math', { latex: e.latex.slice(0, 40) }) })),
    ...audioGate(state.draft, state.meta.grade_levels, state.meta.audio_exempt_reason).map(gateMsg),
    ...originalMissing(state.draft, lang).map(gateMsg),
    ...mediaGate(state.draft, mediaStatus).map(gateMsg),
    ...server422.map(gateMsg),
  ];
  const partial = translationPartial(state.draft, lang) || server?.translation_status === 'partial';
  const warnings = partial ? [t('content_studio.gate.partial')] : [];

  const fail = (e: unknown) => {
    if (e instanceof ApiError) {
      const g = serverGates(e);
      if (g.length) setServer422(g);
      if (e.code === 'CONTENT_NOT_EDITABLE') void reload();
    }
    setActionError(e);
  };

  const create = useIdempotentMutation(async (v: { body: Record<string, unknown>; headers: Record<string, string> }, intent) => qapi.create(v.body, { ...intent, ...v.headers }));

  const onAction = async (a: WorkflowAction) => {
    setActionError(null);
    if (a === 'reject' || a === 'publish') return setDialog(a);
    setBusy(a);
    try {
      if (!questionId) {
        // first save creates the question (System Admin: owner chosen first — F053-AC-12)
        let owner = ownerOf(null);
        if (access.crossTenant) {
          const v = await ownerForm.validateFields().catch(() => null);
          if (!v) return;
          owner = ownerOf(v[OWNER_FIELD]);
        }
        const built = await buildMathAssets(collectMath({ stem: state.draft.stem, render_payload: state.draft.render_payload, explanation: state.draft.explanation, hints: state.draft.hints }, lang));
        const r = await create.mutateAsync({ body: { ...toAuthoringBody(state.draft, state.meta, built.assets, mediaRefs(state.draft)), ...owner.body }, headers: owner.headers });
        qapi.remember(r.data.content_id, r);
        navigate(`../${r.data.content_id}`, { replace: true });
        return;
      }
      if (a === 'save') {
        await autosave.saveNow();
        return;
      }
      if (a === 'new_draft') {
        const r = await qapi.newVersion(questionId, server!.version, { 'x-ew-intent': `new-version:${questionId}:${server!.version}` });
        qapi.remember(questionId, r);
        await reload();
        return;
      }
      if (a === 'submit_review') {
        await autosave.saveNow();
        const now = autosave.current();
        if (now.state === 'error' || now.state === 'conflict' || now.dirty) return; // not saved: nothing submitted
      }
      await transition.mutateAsync({ contentType: 'question', contentId: questionId, version: server!.version, action: a as 'submit_review' | 'approve', etag: autosave.current().etag });
      await reload();
    } catch (e) {
      fail(e);
    } finally {
      setBusy(null);
    }
  };

  const mediaSlot = (args: MediaSlotArgs) => (
    <MediaPicker
      id={args.id}
      kind={args.kind}
      value={args.value}
      onChange={args.onChange}
      onStatus={(id, s) => setMediaStatus((m) => (m[id] === s ? m : { ...m, [id]: s }))}
      disabled={readOnly}
      limitReached={mediaRefs(state.draft).length >= ASSETS_PER_VERSION_MAX}
    />
  );

  const title = questionId ? t('content_studio.question.editTitle') : t('content_studio.question.newTitle');
  return (
    <>
      {!wide && <LargeScreenRequired />}
      {actionError ? <ProblemMessage error={actionError} /> : null}
      <ContentEditorShell
        title={title}
        badges={
          <>
            {assigned && <AssignedBadge />}
            {server?.translation_status && <TranslationBadge status={server.translation_status === 'complete' ? 'complete' : server.translation_status === 'outdated' ? 'outdated' : 'partial'} />}
          </>
        }
        dirty={dirtyRef.current}
        registerDirtyGuard={registerGuard}
        conflict={autosave.state === 'conflict' ? { onReload: () => void reload(), onCopyMine: () => void navigator.clipboard?.writeText(JSON.stringify(state, null, 2)).catch(() => undefined) } : null}
        statusBar={{
          status,
          version: server?.version ?? 1,
          saveState: autosave.state,
          lastSavedAt: autosave.lastSavedAt,
          can: { write: access.question.write, review: access.question.review, publish: access.question.publish },
          isAuthor: !!server?.author_id && server.author_id === me.data?.user_id,
          readOnly: !wide || assigned,
          blockers,
          warnings,
          busy,
          onAction: (a) => void onAction(a),
          onJumpTo: jumpToField,
          wrapAction: (a, b) => (!questionId && a !== 'save' ? null : gateQuestionAction(a, b)),
        }}
        aside={
          <Tabs
            items={[
              { key: 'meta', label: t('content_studio.question.meta'), children: <QuestionMetaPanel value={state.meta} onChange={(meta) => change({ ...state, meta })} readOnly={readOnly} languageLocked={!!questionId} errors={errors} /> },
              { key: 'preview', label: t('content_studio.preview.title'), children: <StudentViewPreview draft={state.draft} contentLanguage={lang} saved={questionId && server ? { contentId: questionId, version: server.version } : undefined} dirty={autosave.dirty} /> },
              ...(questionId && server?.content_version_id && status === 'published'
                ? [{ key: 'regrade', label: t('content_studio.regrade.title'), children: <RegradeEntryButton scope="question_version" versionId={server.content_version_id} item={{ contentType: 'question', contentId: questionId }} /> }]
                : []),
            ]}
          />
        }
      >
        {!questionId && access.crossTenant && (
          <Card size="small">
            <Form form={ownerForm} layout="vertical">
              <OwnerScopeField />
            </Form>
          </Card>
        )}
        {status === 'published' && <Alert type="info" showIcon message={t('content_studio.question.publishedReadOnly', { version: server?.version })} />}
        {questionId && server && EDITABLE.has(status) && <ReviewFieldComments item={{ contentType: 'question', contentId: questionId }} version={server.version} />}
        <QuestionEditor
          value={state.draft}
          onChange={(draft) => change({ ...state, draft })}
          contentLanguage={lang}
          readOnly={readOnly}
          errors={errors}
          outdatedFields={outdated}
          mediaSlot={mediaSlot}
          stemAudioRequired={audioGate(state.draft, state.meta.grade_levels, state.meta.audio_exempt_reason).length > 0}
          typeLocked={!!questionId}
        />
      </ContentEditorShell>
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
            .mutateAsync({ contentType: 'question', contentId: questionId!, version: server!.version, action: 'reject', note, etag: autosave.current().etag })
            .then(reload, fail)
            .finally(() => setBusy(null));
        }}
      />
      <PublishDialog
        open={dialog === 'publish'}
        partial={partial}
        loading={publish.isPending}
        onCancel={() => setDialog(null)}
        onPublish={(changeNote) => {
          setBusy('publish');
          publish
            .mutateAsync({ contentType: 'question', contentId: questionId!, version: server!.version, changeNote, etag: autosave.current().etag })
            .then(async () => {
              setDialog(null);
              await reload();
            }, fail)
            .finally(() => setBusy(null));
        }}
      />
    </>
  );
}

/** `422 MATH_RENDER_FAILED` (`/math_assets/<i>`) → the fields holding that formula (F053-AC-18). */
function mathGates(e: ApiError, built: MathBuildResult): Gate[] {
  const out: Gate[] = [];
  for (const pointer of Object.keys(e.fieldErrors)) {
    const m = /^\/math_assets\/(\d+)/.exec(pointer);
    const paths = m ? (built.assetPaths[Number(m[1])] ?? []) : [];
    if (paths.length === 0) out.push({ field: '', key: 'content_studio.problem.MATH_RENDER_FAILED' });
    for (const p of paths) out.push({ field: fieldOf(p), key: 'content_studio.problem.MATH_RENDER_FAILED' });
  }
  return out.length ? out : [{ field: '', key: 'content_studio.problem.MATH_RENDER_FAILED' }];
}

function PublishDialog({ open, partial, loading, onCancel, onPublish }: { open: boolean; partial: boolean; loading: boolean; onCancel: () => void; onPublish: (changeNote?: string) => void }) {
  const { t } = useTranslation();
  const [note, setNote] = useState('');
  return (
    <Modal open={open} title={t('content_studio.question.publishTitle')} okText={t('content_studio.question.publishOk')} cancelText={t('content_studio.cancel')} confirmLoading={loading} onCancel={onCancel} onOk={() => onPublish(note.trim() || undefined)} destroyOnHidden>
      <Flex vertical gap="small">
        {partial && <Alert type="warning" showIcon message={t('content_studio.gate.partial')} />}
        <Input.TextArea aria-label={t('content_studio.question.changeNote')} placeholder={t('content_studio.question.changeNote')} maxLength={LIMITS.changeNote} showCount value={note} onChange={(e) => setNote(e.target.value)} />
      </Flex>
    </Modal>
  );
}
