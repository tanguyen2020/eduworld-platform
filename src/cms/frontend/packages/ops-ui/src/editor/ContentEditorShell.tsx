// ContentEditorShell (ADMIN_PORTAL §1.4, F053 T015): common frame of every versioned-content editor (questions in
// F053; lessons, activities, quests… later): title + badges, ItemStatusBar, ConflictBanner, form body and an optional
// side panel (preview). While the draft is dirty it asks the browser to confirm closing the tab and registers a
// dirty probe with the host (the portal's unsavedChangesGuard: switching tenant / signing out asks first).
import { Card, Col, Flex, Row, theme, Typography } from 'antd';
import { useEffect, useRef, type ReactNode } from 'react';
import { ConflictBanner } from './ConflictBanner';
import { ItemStatusBar, type ItemStatusBarProps } from './ItemStatusBar';

export interface ContentEditorShellProps {
  title: ReactNode;
  /** Badges next to the title ("Từ EduWorld", translation status…). */
  badges?: ReactNode;
  statusBar: ItemStatusBarProps;
  conflict?: { onReload: () => void; onCopyMine?: () => void } | null;
  dirty: boolean;
  /** Host guard registration; returns the unregister function. */
  registerDirtyGuard?: (isDirty: () => boolean) => () => void;
  aside?: ReactNode;
  children: ReactNode;
}

export function ContentEditorShell({ title, badges, statusBar, conflict, dirty, registerDirtyGuard, aside, children }: ContentEditorShellProps) {
  const { token } = theme.useToken();
  const dirtyRef = useRef(dirty);
  dirtyRef.current = dirty;

  useEffect(() => (registerDirtyGuard ? registerDirtyGuard(() => dirtyRef.current) : undefined), [registerDirtyGuard]);
  useEffect(() => {
    const onBeforeUnload = (e: BeforeUnloadEvent) => {
      if (!dirtyRef.current) return;
      e.preventDefault();
      e.returnValue = '';
    };
    window.addEventListener('beforeunload', onBeforeUnload);
    return () => window.removeEventListener('beforeunload', onBeforeUnload);
  }, []);

  return (
    <Flex vertical gap="middle">
      <Flex align="center" gap="small" wrap>
        <Typography.Title level={1}>{title}</Typography.Title>
        {badges}
      </Flex>
      <ItemStatusBar {...statusBar} />
      {conflict && <ConflictBanner onReload={conflict.onReload} onCopyMine={conflict.onCopyMine} />}
      <Row gutter={[token.margin, token.margin]}>
        <Col xs={24} xl={aside ? 16 : 24}>
          <Card>{children}</Card>
        </Col>
        {aside && (
          <Col xs={24} xl={8}>
            {aside}
          </Col>
        )}
      </Row>
    </Flex>
  );
}
