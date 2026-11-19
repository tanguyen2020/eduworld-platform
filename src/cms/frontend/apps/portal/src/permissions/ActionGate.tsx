// ActionGate (F051 T061, F051-AC-36, ADR-0090 mục 3): wraps a button/operation of the catalog. `actionKey` is the
// generated `ActionKey` union, so an unknown key does not compile. Hidden when the user has none of the required
// permissions or the action (or its menu) is turned off; `mode="disable"` keeps a turned-off action visible but
// disabled with the tooltip "Tính năng đang tắt". Display only — the backend answers 403 FEATURE_DISABLED anyway.
import { Tooltip } from 'antd';
import { cloneElement, isValidElement, type ReactElement, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import type { ActionKey } from '../generated/menu-catalog';
import { actionStatus } from './menuRegistry';
import { usePermissions } from './usePermissions';

export interface ActionGateProps {
  actionKey: ActionKey;
  mode?: 'hide' | 'disable';
  children: ReactNode;
}

export function ActionGate({ actionKey, mode = 'hide', children }: ActionGateProps) {
  const { t } = useTranslation();
  const { view } = usePermissions();
  const status = view ? actionStatus(actionKey, view) : 'forbidden';
  if (status === 'enabled') return <>{children}</>;
  if (status === 'disabled' && mode === 'disable' && isValidElement(children)) {
    return (
      <Tooltip title={t('shell.featureDisabled.title')}>
        <span data-action-disabled={actionKey}>{cloneElement(children as ReactElement<{ disabled?: boolean }>, { disabled: true })}</span>
      </Tooltip>
    );
  }
  return null;
}
