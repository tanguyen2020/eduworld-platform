// ConfigMenuScreen (F079 T001, T044): the screen of one game-config menu of the catalog (`menu.game_ops.<menu>`,
// ADR-0090 — menus and paths come from the generated catalog, no hand-written menu): its config types as tabs, list
// `:configType`, new `:configType/new` (Event Builder for events), editor `:configType/:configId/v/:version`, and the
// event calendar (`calendar`) under Sự kiện.
import type { ComponentType } from 'react';
import { useTranslation } from 'react-i18next';
import { Navigate, Route, Routes } from 'react-router-dom';
import { NotFound } from '../../app/NotFound';
import { EventCalendarPage } from './builders/event/EventCalendarPage';
import { ConfigEditorPage } from './editor/ConfigEditorPage';
import { CreateConfigPage } from './editor/CreateConfigPage';
import { ConfigListPage } from './list/ConfigListPage';
import { typesOfMenu, type GameOpsMenu } from './registry/configTypes';

function MenuRoutes({ title, menu }: { title: string; menu: GameOpsMenu }) {
  const { t } = useTranslation();
  const types = typesOfMenu(menu);
  const calendar = menu === 'events' ? [{ key: 'calendar', label: t('gameOps.calendar.tab') }] : [];
  return (
    <Routes>
      <Route index element={<Navigate to={types[0]!.type} replace />} />
      {menu === 'events' && <Route path="calendar" element={<EventCalendarPage title={title} />} />}
      <Route path=":configType" element={<ConfigListPage title={title} menu={menu} extraTabs={calendar} />} />
      <Route path=":configType/new" element={<CreateConfigPage />} />
      <Route path=":configType/:configId/v/:version" element={<ConfigEditorPage />} />
      <Route path="*" element={<NotFound />} />
    </Routes>
  );
}

const cache = new Map<GameOpsMenu, ComponentType<{ title: string }>>();

/** Screen component of a menu (stable identity per menu). */
export function configMenuScreen(menu: GameOpsMenu): ComponentType<{ title: string }> {
  let c = cache.get(menu);
  if (!c) {
    const Screen = ({ title }: { title: string }) => <MenuRoutes title={title} menu={menu} />;
    Screen.displayName = `ConfigMenuScreen(${menu})`;
    c = Screen;
    cache.set(menu, c);
  }
  return c;
}
