// ItemStatusBar (F053-AC-06, AC-08, AC-09, AC-10, AC-13, AC-23, AC-24; data-model §2): status, version, save state
// and the workflow actions of ONE item. Buttons follow the status and the permissions passed by the page (hidden
// when not allowed — the backend decides anyway, AGENTS R20): draft/rejected → Lưu, Gửi duyệt; in_review → Duyệt,
// Từ chối (never for the author of the version); approved → Xuất bản ngay; published → Tạo bản nháp từ v{n}.
// Blockers (missing required fields, audio, media not ready, formula errors) disable Gửi duyệt / Xuất bản and are
// listed as links that jump to the field. Presentational: no API call (ADMIN_PORTAL §1.1).
import { Alert, Button, Flex, Space, Typography } from 'antd';
import dayjs from 'dayjs';
import type { ReactElement, ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { DomainTag } from '../components/DomainTag/DomainTag';
import { OPS_UI_NS } from '../i18n';
import type { SaveState } from './useAutosave';

export type ContentStatusValue = 'draft' | 'in_review' | 'approved' | 'published' | 'archived' | 'rejected';
export type WorkflowAction = 'save' | 'submit_review' | 'approve' | 'reject' | 'publish' | 'new_draft';

export interface EditorBlocker {
  /** Field path (JSON pointer or form path) to jump to. */
  field: string;
  /** Translated message. */
  message: string;
}

export interface ItemStatusBarProps {
  status: ContentStatusValue;
  version: number;
  saveState?: SaveState;
  lastSavedAt?: Date | null;
  can: { write: boolean; review: boolean; publish: boolean };
  /** The signed-in user authored this version (reviewer ≠ author). */
  isAuthor: boolean;
  /** Assigned platform content, small screen…: no write action at all. */
  readOnly?: boolean;
  blockers?: readonly EditorBlocker[];
  /** Non-blocking notices (e.g. translation partial). */
  warnings?: readonly string[];
  busy?: WorkflowAction | null;
  onAction: (action: WorkflowAction) => void;
  onJumpTo?: (field: string) => void;
  /** Wraps each action button (the portal passes `<ActionGate actionKey>`, ADR-0090). */
  wrapAction?: (action: WorkflowAction, button: ReactElement) => ReactNode;
}

/** Actions offered for a status (before permissions), in display order. */
export function actionsForStatus(status: ContentStatusValue): WorkflowAction[] {
  switch (status) {
    case 'draft':
    case 'rejected':
      return ['save', 'submit_review'];
    case 'in_review':
      return ['approve', 'reject'];
    case 'approved':
      return ['publish'];
    case 'published':
      return ['new_draft'];
    default:
      return [];
  }
}

/** Actions visible to this user (permissions, author rule, read-only). */
export function visibleActions(p: Pick<ItemStatusBarProps, 'status' | 'can' | 'isAuthor' | 'readOnly'>): WorkflowAction[] {
  if (p.readOnly) return [];
  return actionsForStatus(p.status).filter((a) => {
    if (a === 'save' || a === 'submit_review' || a === 'new_draft') return p.can.write;
    if (a === 'approve' || a === 'reject') return p.can.review && !p.isAuthor;
    if (a === 'publish') return p.can.publish;
    return false;
  });
}

function SaveStateText({ state, lastSavedAt }: { state: SaveState; lastSavedAt?: Date | null }) {
  const { t } = useTranslation(OPS_UI_NS);
  if (state === 'saving') return <Typography.Text type="secondary">{t('editor.save.saving')}</Typography.Text>;
  if (state === 'error') return <Typography.Text type="danger">{t('editor.save.error')}</Typography.Text>;
  if (state === 'conflict') return <Typography.Text type="danger">{t('editor.save.conflict')}</Typography.Text>;
  if (state === 'saved' && lastSavedAt) return <Typography.Text type="secondary">{t('editor.save.savedAt', { time: dayjs(lastSavedAt).format('HH:mm') })}</Typography.Text>;
  return null;
}

export function ItemStatusBar(props: ItemStatusBarProps) {
  const { t } = useTranslation(OPS_UI_NS);
  const { status, version, saveState = 'idle', lastSavedAt, blockers = [], warnings = [], busy, onAction, onJumpTo, wrapAction } = props;
  const actions = visibleActions(props);
  const blocked = blockers.length > 0;
  const button = (a: WorkflowAction) => {
    const primary = a === 'submit_review' || a === 'approve' || a === 'publish' || a === 'new_draft';
    const disabled = (blocked && (a === 'submit_review' || a === 'publish')) || (!!busy && busy !== a) || (a === 'save' && saveState === 'conflict');
    const node = (
      <Button key={a} type={primary ? 'primary' : 'default'} danger={a === 'reject'} loading={busy === a} disabled={disabled} onClick={() => onAction(a)} data-action={a}>
        {a === 'new_draft' ? t('editor.action.new_draft', { version }) : t(`editor.action.${a}`)}
      </Button>
    );
    return <span key={a}>{wrapAction ? wrapAction(a, node) : node}</span>;
  };
  return (
    <Flex vertical gap="small" role="region" aria-label={t('editor.statusBar')}>
      <Flex justify="space-between" align="center" wrap gap="small">
        <Space wrap>
          <DomainTag kind="content_status" value={status} />
          <Typography.Text strong>{status === 'published' ? t('editor.publishedVersion', { version }) : t('editor.version', { version })}</Typography.Text>
          {props.readOnly && <Typography.Text type="secondary">{t('editor.readOnly')}</Typography.Text>}
          <SaveStateText state={saveState} lastSavedAt={lastSavedAt} />
          {saveState === 'error' && (
            <Button size="small" type="link" onClick={() => onAction('save')}>
              {t('editor.save.retry')}
            </Button>
          )}
        </Space>
        <Space wrap>{actions.map(button)}</Space>
      </Flex>
      {blocked && (status === 'draft' || status === 'rejected' || status === 'approved') && (
        <Alert
          type="warning"
          showIcon
          message={t('editor.blockers.title')}
          description={
            <ul aria-label={t('editor.blockers.title')}>
              {blockers.map((b) => (
                <li key={`${b.field}:${b.message}`}>
                  <Button type="link" size="small" onClick={() => onJumpTo?.(b.field)}>
                    {b.message}
                  </Button>
                </li>
              ))}
            </ul>
          }
        />
      )}
      {warnings.map((w) => (
        <Alert key={w} type="info" showIcon message={w} />
      ))}
    </Flex>
  );
}
