// ConfigEditorPage (F079-AC-02, AC-03, AC-04, AC-05, AC-16, AC-17; ADMIN_PORTAL §1.4, §4): one version of a game config in
// the shared ContentEditorShell — title + payload form of the type's builder, autosave of the draft (updateGameConfigDraft,
// If-Match = last ETag; `412` / `409 CONTENT_VERSION_CONFLICT` stops autosave and offers "Tải bản mới"), workflow buttons
// (submit_review, approve / reject with note — hidden for the author, the server enforces reviewer ≠ author — publish now
// or scheduled, new draft from a published version) each inside ActionGate, `422` field errors on their field and as
// blockers of "Gửi duyệt", tabs Phiên bản / Lịch sử phát hành, thumbnail / turntable of THIS version and the Godot Web
// preview. Below 1024 px the version is read only ("Tác vụ này cần màn hình lớn hơn").
import { ContentEditorShell, ThumbnailImage, TurntableViewer, useAutosave, type WorkflowAction } from '@eduworld/ops-ui/game-console';
import { Alert, App as AntApp, Card, Flex, Form, Input, Skeleton, Tabs } from 'antd';
import { useState, type ReactElement } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate, useParams } from 'react-router-dom';
import { usePortalRuntime } from '../../../app/runtime';
import { ActionGate } from '../../../permissions/ActionGate';
import { CONTENT_AUTOSAVE_DEBOUNCE_MS, CONTENT_AUTOSAVE_INTERVAL_MS, EDITOR_MIN_WIDTH_QUERY } from '../../../shared/release/config';
import { ReasonDialog } from '../../../shell/ReasonDialog';
import { useMediaQuery } from '../../../shell/useMediaQuery';
import { isConflict, saveDraft, useConfigVersion, useConfigWrites, type GameConfigVersion, type ItemRef, type Loaded } from '../api/gameConfigs';
import { ConfigBuilder } from '../builders/ConfigBuilder';
import { actionOf, CONFIG_TYPES, isConfigType, nameOf, type ConfigTypeDef } from '../registry/configTypes';
import { useGameAccess } from '../shared/access';
import { fieldErrorsOf, GameOpsError, hasFieldErrors } from '../shared/errors';
import { AssignmentsPanel } from './AssignmentsPanel';
import { PreviewPanel } from './PreviewPanel';
import { PublishDialog } from './PublishDialog';
import { ReleasesPanel } from './ReleasesPanel';
import { VersionsPanel } from './VersionsPanel';

/** GameConfigInput.title maxLength (OpenAPI). */
export const TITLE_MAX = 200;
/** transitionGameConfigVersion note maxLength (OpenAPI). */
export const NOTE_MAX = 2000;

interface Draft {
  title?: string;
  payload: Record<string, unknown>;
}

export function ConfigEditorPage() {
  const { configType, configId, version } = useParams();
  const [generation, setGeneration] = useState(0);
  if (!isConfigType(configType) || !configId || !version) return <GameOpsError error={null} />;
  const def = CONFIG_TYPES[configType];
  return <LoadedEditor key={`${configId}/${version}/${generation}`} def={def} item={{ configType, configId }} version={Number(version)} onReload={() => setGeneration((g) => g + 1)} />;
}

function LoadedEditor({ def, item, version, onReload }: { def: ConfigTypeDef; item: ItemRef; version: number; onReload: () => void }) {
  const q = useConfigVersion(item, version);
  if (q.isPending) return <Skeleton active />;
  if (q.error) return <GameOpsError error={q.error} />;
  return <EditorBody def={def} item={item} version={version} loaded={q.data} onReload={async () => (await q.refetch(), onReload())} />;
}

function EditorBody({ def, item, version, loaded, onReload }: { def: ConfigTypeDef; item: ItemRef; version: number; loaded: Loaded<GameConfigVersion>; onReload: () => void }) {
  const { t, i18n } = useTranslation();
  const { message } = AntApp.useApp();
  const { api } = usePortalRuntime();
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const access = useGameAccess(def);
  const wide = useMediaQuery(EDITOR_MIN_WIDTH_QUERY);
  const writes = useConfigWrites();
  const v = loaded.data;
  const lang = i18n.language.startsWith('en') ? 'en' : 'vi';
  const [draft, setDraft] = useState<Draft>({ title: v.title, payload: v.payload });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [lastError, setLastError] = useState<unknown>(null);
  const [revision, setRevision] = useState<string | null>(loaded.etag);
  const [rejecting, setRejecting] = useState(false);
  const [publishing, setPublishing] = useState<number | null>(null);
  const [busy, setBusy] = useState<WorkflowAction | null>(null);
  const status = v.status;
  const editable = (status === 'draft' || status === 'rejected') && access.write && wide;
  const exists = (k: string) => i18n.exists(k);

  const autosave = useAutosave<Draft>(draft, editable, {
    save: async (value, etag) => {
      try {
        const r = await saveDraft(api, item, version, value, etag);
        setErrors({});
        setLastError(null);
        setRevision(r.etag);
        return { etag: r.etag };
      } catch (e) {
        setLastError(e);
        if (hasFieldErrors(e)) setErrors(fieldErrorsOf(e, t, exists));
        throw e;
      }
    },
    isConflict,
    intervalMs: CONTENT_AUTOSAVE_INTERVAL_MS,
    debounceMs: CONTENT_AUTOSAVE_DEBOUNCE_MS,
    initialEtag: loaded.etag,
  });

  const transition = (action: 'submit_review' | 'approve' | 'reject', note?: string) => {
    setBusy(action);
    writes.transition.mutate(
      { ...item, version, action, ...(note ? { note } : {}) },
      {
        onSuccess: () => {
          setErrors({});
          message.success(t(`gameOps.editor.done.${action}`));
          onReload();
        },
        onError: (e) => {
          setLastError(e);
          setErrors(fieldErrorsOf(e, t, exists));
        },
        onSettled: () => setBusy(null),
      },
    );
  };

  const onAction = async (a: WorkflowAction) => {
    if (a === 'save') return void autosave.saveNow();
    if (a === 'submit_review') {
      setBusy(a);
      await autosave.saveNow();
      const s = autosave.current();
      setBusy(null);
      if (s.state === 'error' || s.state === 'conflict' || s.dirty) return;
      return transition('submit_review');
    }
    if (a === 'approve') return transition('approve');
    if (a === 'reject') return setRejecting(true);
    if (a === 'publish') return setPublishing(version);
    if (a === 'new_draft') {
      setBusy(a);
      writes.newVersion.mutate(
        { ...item, basedOn: version },
        { onSuccess: (nv) => navigate(itemPath(nv.version)), onError: (e) => setLastError(e), onSettled: () => setBusy(null) },
      );
    }
  };

  const itemPath = (no: number) => pathname.replace(/\/v\/\d+$/, `/v/${no}`);
  const ACTION_KIND: Record<WorkflowAction, 'edit' | 'workflow' | 'publish'> = { save: 'edit', submit_review: 'workflow', approve: 'workflow', reject: 'workflow', publish: 'publish', new_draft: 'edit' };
  const wrapAction = (a: WorkflowAction, button: ReactElement) => (
    <ActionGate actionKey={actionOf(def, ACTION_KIND[a])} mode="disable">
      {button}
    </ActionGate>
  );
  const blockers = Object.entries(errors).map(([field, msg]) => ({ field, message: t('gameOps.editor.blocker', { field, message: msg }) }));
  const displayName = draft.title || nameOf(def, draft.payload, lang) || t('gameOps.common.untitled');

  return (
    <>
      {!wide && <Alert type="info" showIcon message={t('shell.largeScreen.title')} description={t('shell.largeScreen.description')} data-testid="large-screen-required" />}
      <ContentEditorShell
        title={`${t(`gameOps.type.${def.type}`)} · ${displayName}`}
        statusBar={{
          status,
          version,
          saveState: autosave.state,
          lastSavedAt: autosave.lastSavedAt,
          can: { write: access.write && wide, review: access.review, publish: access.publish },
          isAuthor: !!v.author_id && v.author_id === access.userId,
          blockers,
          busy,
          onAction: (a) => void onAction(a),
          onJumpTo: (field) => document.getElementById(`gf-${def.type}-${field.replace(/[./]/g, '-')}`)?.scrollIntoView?.(),
          wrapAction,
        }}
        conflict={autosave.state === 'conflict' ? { onReload } : null}
        dirty={autosave.dirty}
        aside={
          <Flex vertical gap="middle">
            {def.has3D && (
              <Card size="small" title={t('gameOps.editor.render')} data-testid="editor-render">
                <Flex vertical gap="small" align="center">
                  <ThumbnailImage src={v.thumbnail_url} status={v.render_status} alt={t('gameOps.editor.thumbnailAlt', { name: displayName, version })} size="large" />
                  {v.turntable_url && v.render_status === 'ready' && <TurntableViewer src={v.turntable_url} alt={t('gameOps.editor.turntableAlt', { name: displayName, version })} />}
                </Flex>
              </Card>
            )}
            {def.preview !== false && <PreviewPanel configType={item.configType} configId={item.configId} version={version} />}
            {def.assignable && <AssignmentsPanel resourceType={def.assignable} resourceId={item.configId} />}
          </Flex>
        }
      >
        <Flex vertical gap="middle">
          {lastError && !isConflict(lastError) ? <GameOpsError error={lastError} /> : null}
          {v.review_note && status === 'rejected' && <Alert type="warning" showIcon message={t('gameOps.editor.reviewNote', { note: v.review_note })} />}
          <Tabs
            items={[
              {
                key: 'content',
                label: t('gameOps.editor.tab.content'),
                children: (
                  <Flex vertical gap="middle">
                    <Form layout="vertical" component="div">
                      <Form.Item label={t('gameOps.editor.title')} htmlFor="game-config-title">
                        <Input id="game-config-title" maxLength={TITLE_MAX} showCount disabled={!editable} value={draft.title ?? ''} onChange={(e) => setDraft({ ...draft, title: e.target.value || undefined })} />
                      </Form.Item>
                    </Form>
                    <ConfigBuilder
                      def={def}
                      value={draft.payload}
                      onChange={(payload) => setDraft({ ...draft, payload })}
                      readOnly={!editable}
                      errors={errors}
                      item={{ configId: item.configId, version, revision }}
                    />
                  </Flex>
                ),
              },
              { key: 'versions', label: t('gameOps.editor.tab.versions'), children: <VersionsPanel item={item} editorPath={itemPath} /> },
              { key: 'releases', label: t('gameOps.editor.tab.releases'), children: <ReleasesPanel item={item} /> },
            ]}
          />
        </Flex>
      </ContentEditorShell>
      <ReasonDialog
        open={rejecting}
        title={t('gameOps.editor.rejectTitle')}
        maxLength={NOTE_MAX}
        confirmLoading={writes.transition.isPending}
        onCancel={() => setRejecting(false)}
        onSubmit={(note) => {
          setRejecting(false);
          transition('reject', note);
        }}
      />
      <PublishDialog item={item} version={publishing} onClose={() => (setPublishing(null), onReload())} />
    </>
  );
}
