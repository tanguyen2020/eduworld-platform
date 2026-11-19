// DocumentEditorPage (F054-AC-20…AC-23, ADMIN_PORTAL §1.4, ADR-0043): the frame shared by the Question Set, Lesson and
// Assessment Blueprint builders on ContentEditorShell (F053) — load the latest version, autosave the draft (merge
// patch + If-Match), workflow of the item (submit / approve / reject; publish now or scheduled through PublishDialog),
// `CONTENT_NOT_EDITABLE` → read only with "Tạo bản nháp từ phiên bản này", create with the owner chosen by a System
// Admin, release tabs (versions, releases, usages, preview) under the editor. Builders ≥ 1024 px; smaller screens read.
// The page orchestrates only; rules live in module learning (AGENTS R1).
import { ContentEditorShell, useAutosave, type EditorBlocker, type WorkflowAction } from '@eduworld/ops-ui/content';
import { Alert, Button, Card, Form, Result, Spin } from 'antd';
import { useCallback, useMemo, useRef, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { registerUnsavedChanges } from '../../../session/unsavedChangesGuard';
import { useMe } from '../../../shared/api/hooks/useMe';
import { ApiError } from '../../../shared/api/middleware/problem';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { ActionGate } from '../../../permissions/ActionGate';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { ReasonDialog } from '../../../shell/ReasonDialog';
import { LargeScreenRequired } from '../../../shell/StatusPages';
import { useMediaQuery } from '../../../shell/useMediaQuery';
import { useWorkflow } from '../questions/useWorkflow';
import { ContentReleaseTabs } from '../release/ContentReleaseTabs';
import { PublishDialog } from '../release/PublishDialog';
import { useRelease } from '../release/useRelease';
import { useContentAccess } from '../shared/access';
import { AssignedBadge } from '../shared/AssignedBadge';
import { CONTENT_AUTOSAVE_DEBOUNCE_MS, CONTENT_AUTOSAVE_INTERVAL_MS, EDITOR_MIN_WIDTH_QUERY, LIMITS } from '../shared/config';
import { CONTENT_TYPES } from '../shared/contentTypes';
import { OWNER_FIELD, ownerOf, OwnerScopeField } from '../shared/OwnerScopeField';
import { useDocumentApi, useDocumentQuery, type DocAuthoring, type DocType } from './useDocument';

const EDITABLE = new Set(['draft', 'rejected']);
const isConflict = (e: unknown) => e instanceof ApiError && (e.status === 412 || e.code === 'CONTENT_VERSION_CONFLICT' || e.code === 'VERSION_MISMATCH');

export interface BuilderArgs<S> {
  state: S;
  onChange: (s: S) => void;
  readOnly: boolean;
  /** Field errors of the last server answer (JSON pointer → message). */
  errors: Readonly<Record<string, string>>;
  server?: DocAuthoring;
}

export interface DocumentEditorPageProps<S> {
  type: DocType;
  id?: string;
  title: { create: string; edit: string };
  empty: () => S;
  fromServer: (d: DocAuthoring) => S;
  /** Authoring body of the state (formulas rendered first when the type has math_assets). */
  toBody: (s: S) => Promise<Record<string, unknown>>;
  /** Local blockers of Gửi duyệt (pure validation of the builder). */
  blockers?: (s: S) => EditorBlocker[];
  body: (a: BuilderArgs<S>) => ReactNode;
  aside?: (a: BuilderArgs<S>) => ReactNode;
}

export function DocumentEditorPage<S>(props: DocumentEditorPageProps<S>) {
  const { t } = useTranslation();
  const q = useDocumentQuery(props.type, props.id);
  const [nonce, setNonce] = useState(0);
  if (props.id && q.isPending) return <Spin aria-label={t('content_studio.loading')} />;
  if (props.id && q.error) return q.error instanceof ApiError && q.error.status === 404 ? <Result status="404" title={t('content_studio.notFound')} /> : <ProblemMessage error={q.error} />;
  const loaded = q.data;
  return (
    <DocumentEditorForm
      key={`${props.id ?? 'new'}:${loaded?.data.version ?? 0}:${loaded?.etag ?? ''}:${nonce}`}
      {...props}
      server={loaded?.data}
      etag={loaded?.etag ?? null}
      reload={async () => {
        await q.refetch();
        setNonce((n) => n + 1);
      }}
    />
  );
}

function DocumentEditorForm<S>({ type, id, title, empty, fromServer, toBody, blockers, body, aside, server, etag, reload }: DocumentEditorPageProps<S> & { server?: DocAuthoring; etag: string | null; reload: () => Promise<void> }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const access = useContentAccess();
  const wide = useMediaQuery(EDITOR_MIN_WIDTH_QUERY);
  const me = useMe();
  const dapi = useDocumentApi(type);
  const { transition } = useWorkflow();
  const { newDraft } = useRelease();
  const initial = useMemo(() => (server ? fromServer(server) : empty()), [server, fromServer, empty]);
  const [state, setState] = useState<S>(initial);
  const [serverErrors, setServerErrors] = useState<Record<string, string>>({});
  const [actionError, setActionError] = useState<unknown>(null);
  const [notEditable, setNotEditable] = useState(false);
  const [busy, setBusy] = useState<WorkflowAction | null>(null);
  const [dialog, setDialog] = useState<'reject' | 'publish' | null>(null);
  const [ownerForm] = Form.useForm<{ [OWNER_FIELD]?: string }>();
  const status = server?.status ?? 'draft';
  const assigned = !!server?.assigned;
  const actions = CONTENT_TYPES[type].actions;
  const readOnly = !wide || assigned || notEditable || !access.question.write || (!!server && !EDITABLE.has(status));
  const local = useMemo(() => blockers?.(state) ?? [], [blockers, state]);

  const capture = (e: unknown) => {
    if (!(e instanceof ApiError)) return;
    if (e.code === 'CONTENT_NOT_EDITABLE') setNotEditable(true);
    const out: Record<string, string> = {};
    for (const [f, fe] of Object.entries(e.fieldErrors)) out[f] = fe.message ?? t(`error.field.${fe.code}`, { defaultValue: t(e.i18nKey) });
    setServerErrors(out);
  };

  const autosave = useAutosave<S>(state, !!id && !readOnly && local.length === 0, {
    intervalMs: CONTENT_AUTOSAVE_INTERVAL_MS,
    debounceMs: CONTENT_AUTOSAVE_DEBOUNCE_MS,
    initialEtag: etag,
    isConflict,
    save: async (v, currentEtag) => {
      try {
        const r = await dapi.saveDraft(id!, server!.version, await toBody(v), currentEtag);
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
  const registerGuard = useCallback((isDirty: () => boolean) => registerUnsavedChanges(`${type}:${id ?? 'new'}`, isDirty), [type, id]);
  const create = useIdempotentMutation(async (v: { body: Record<string, unknown>; headers: Record<string, string> }, intent) => dapi.create(v.body, { ...intent, ...v.headers }));

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
        const r = await create.mutateAsync({ body: { ...(await toBody(state)), ...owner.body }, headers: owner.headers });
        dapi.remember(r.data.content_id, r);
        navigate(`../${r.data.content_id}`, { replace: true });
        return;
      }
      if (a === 'save') return void (await autosave.saveNow());
      if (a === 'new_draft') {
        await newDraft.mutateAsync({ contentType: type, contentId: id, fromVersion: server!.version });
        await reload();
        return;
      }
      if (a === 'submit_review') {
        await autosave.saveNow();
        const now = autosave.current();
        if (now.state === 'error' || now.state === 'conflict' || now.dirty) return;
      }
      await transition.mutateAsync({ contentType: type, contentId: id, version: server!.version, action: a as 'submit_review' | 'approve', etag: autosave.current().etag });
      await reload();
    } catch (e) {
      fail(e);
    } finally {
      setBusy(null);
    }
  };

  const args: BuilderArgs<S> = { state, onChange: setState, readOnly, errors: serverErrors, server };
  const errorBlockers = Object.entries(serverErrors).map(([field, msg]) => ({ field, message: `${field}: ${msg}` }));
  return (
    <>
      {!wide && <LargeScreenRequired />}
      {actionError ? <ProblemMessage error={actionError} /> : null}
      {notEditable && id && server && (
        <Alert
          type="warning"
          showIcon
          message={t('error.CONTENT_NOT_EDITABLE')}
          action={
            actions.edit && (
              <ActionGate actionKey={actions.edit}>
                <Button size="small" onClick={() => void onAction('new_draft')}>
                  {t('content_studio.release.newDraftFrom', { version: server.version })}
                </Button>
              </ActionGate>
            )
          }
        />
      )}
      <ContentEditorShell
        title={id ? title.edit : title.create}
        badges={assigned ? <AssignedBadge /> : null}
        dirty={dirtyRef.current}
        registerDirtyGuard={registerGuard}
        conflict={autosave.state === 'conflict' ? { onReload: () => void reload() } : null}
        statusBar={{
          status,
          version: server?.version ?? 1,
          saveState: autosave.state,
          lastSavedAt: autosave.lastSavedAt,
          can: { write: access.question.write, review: access.question.review, publish: access.question.publish },
          isAuthor: !!server?.author_id && server.author_id === me.data?.user_id,
          readOnly: !wide || assigned,
          blockers: [...local, ...errorBlockers],
          busy,
          onAction: (a) => void onAction(a),
          wrapAction: (a, b) => {
            if (!id && a !== 'save') return null;
            const key = a === 'publish' ? actions.publish : a === 'approve' || a === 'reject' || a === 'submit_review' ? actions.workflow : actions.edit;
            return key ? (
              <ActionGate actionKey={key} mode="disable">
                {b}
              </ActionGate>
            ) : (
              b
            );
          },
        }}
        aside={aside?.(args)}
      >
        {!id && access.crossTenant && (
          <Card size="small">
            <Form form={ownerForm} layout="vertical">
              <OwnerScopeField />
            </Form>
          </Card>
        )}
        {body(args)}
      </ContentEditorShell>
      {id && server && (
        <Card>
          <ContentReleaseTabs item={{ contentType: type, contentId: id }} version={server.version} assigned={assigned} contentLanguage={server.content_language === 'en' ? 'en' : 'vi'} onDraftCreated={() => void reload()} />
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
            .mutateAsync({ contentType: type, contentId: id!, version: server!.version, action: 'reject', note, etag: autosave.current().etag })
            .then(reload, fail)
            .finally(() => setBusy(null));
        }}
      />
      {id && server && (
        <PublishDialog
          item={{ contentType: type, contentId: id }}
          version={dialog === 'publish' ? server.version : null}
          onClose={() => {
            setDialog(null);
            void reload();
          }}
        />
      )}
    </>
  );
}
