// Display permissions of Content Studio (F053-AC-13, AC-29, AC-33; ADMIN_PORTAL §2.3, ADR-0090): which buttons to
// show for the session — learning_content.write / review / publish / translate, media.upload, curriculum.* — and the
// ActionGate of each workflow button (hidden without permission, disabled with "Tính năng đang tắt" when the
// action is turned off). Display only: the backend authorizes every request (AGENTS R20).
import type { ReactElement } from 'react';
import type { WorkflowAction } from '@eduworld/ops-ui/content';
import type { ActionKey } from '../../../generated/menu-catalog';
import { ActionGate } from '../../../permissions/ActionGate';
import { actionStatus } from '../../../permissions/menuRegistry';
import { usePermissions } from '../../../permissions/usePermissions';
import { useTenantSource } from '../../../shared/auth/TenantContext';

export interface ContentAccess {
  ready: boolean;
  crossTenant: boolean;
  /** Platform scope (no session tenant): platform staff, System Admin — platform lane / platform source offered. */
  platform: boolean;
  question: { write: boolean; review: boolean; publish: boolean; translate: boolean; upload: boolean; preview: boolean };
  curriculum: { read: boolean; write: boolean; publish: boolean };
  /** Action enabled (permission held and not turned off). */
  enabled: (key: ActionKey) => boolean;
}

export function useContentAccess(): ContentAccess {
  const { view, crossTenant } = usePermissions();
  const tenant = useTenantSource();
  const perms = view?.permissions ?? [];
  const has = (p: string) => perms.includes(p);
  return {
    ready: !!view,
    crossTenant,
    platform: crossTenant || !tenant.getTenantId(),
    question: {
      write: has('learning_content.write'),
      review: has('learning_content.review'),
      publish: has('learning_content.publish'),
      translate: has('learning_content.translate'),
      upload: has('media.upload'),
      preview: has('learning_content.read'),
    },
    curriculum: { read: has('curriculum.read'), write: has('curriculum.write'), publish: has('curriculum.publish') },
    enabled: (key) => !!view && actionStatus(key, view) === 'enabled',
  };
}

/** Catalog action of each workflow button of the question editor. */
export const QUESTION_ACTION_KEYS: Record<WorkflowAction, ActionKey> = {
  save: 'action.content_studio.question_bank.edit',
  new_draft: 'action.content_studio.question_bank.edit',
  submit_review: 'action.content_studio.question_bank.workflow',
  approve: 'action.content_studio.question_bank.workflow',
  reject: 'action.content_studio.question_bank.workflow',
  publish: 'action.content_studio.question_bank.publish',
};

/** `ItemStatusBar.wrapAction` for questions: every button inside its `<ActionGate>` (disabled when turned off). */
export function gateQuestionAction(action: WorkflowAction, button: ReactElement) {
  return (
    <ActionGate actionKey={QUESTION_ACTION_KEYS[action]} mode="disable">
      {button}
    </ActionGate>
  );
}
