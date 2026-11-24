// EventCalendarPage (F079-AC-14, FR-EVT-002, ADMIN_PORTAL §3.15): the event calendar by month or week in ICT — events of
// the window from listGameConfigs (`events`, `from`/`to` filtered by the server, every page of the window), one bar per
// event colored by its kind (Admin theme tokens), status Tag, and the overlap warnings THE SERVER returns
// (`calendar_warnings`: two decorations in the same area at the same time…) — the portal infers nothing.
import { DomainTag } from '@eduworld/ops-ui';
import { Alert, Button, Calendar, Flex, Segmented, Skeleton, Tag, theme, Typography } from 'antd';
import dayjs, { type Dayjs } from 'dayjs';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { ictDate, utcToIct } from '../../../../shared/release/ict';
import { useEventWindow, type GameConfigVersion } from '../../api/gameConfigs';
import { CONFIG_TYPES, editorPathOf, nameOf } from '../../registry/configTypes';
import { GameOpsError } from '../../shared/errors';

type Schedule = { start_at?: string; end_at?: string };
type Token = ReturnType<typeof theme.useToken>['token'];

/** Admin theme token of an event kind (bar color). */
export function kindColor(kind: string | undefined, token: Token): string {
  const map: Record<string, string> = {
    world_festival: token.colorPrimary,
    weekend: token.colorInfo,
    school: token.colorSuccess,
    community_goal: token.colorWarning,
    learning_campaign: token.colorSuccessText,
    special_boss: token.colorError,
    limited_quest: token.colorPrimaryText,
  };
  return map[kind ?? ''] ?? token.colorTextSecondary;
}

const pad = (n: number) => String(n).padStart(2, '0');
const DAY_LABEL = 'dd DD/MM';
const DAY = 'day';
const isoDay = (d: Dayjs) => `${d.year()}-${pad(d.month() + 1)}-${pad(d.date())}`;

/** ICT days (yyyy-MM-dd) covered by an event schedule. */
export function eventDays(s: Schedule): { from: string; to: string } | null {
  if (!s.start_at || !s.end_at) return null;
  return { from: ictDate(new Date(s.start_at)), to: ictDate(new Date(s.end_at)) };
}

/** Window of the view: the month of `d`, or its Monday–Sunday week (ICT days). */
export function windowOf(d: Dayjs, mode: 'month' | 'week'): { from: string; to: string } {
  if (mode === 'month') return { from: isoDay(d.startOf('month')), to: isoDay(d.endOf('month')) };
  const monday = d.subtract((d.day() + 6) % 7, 'day');
  return { from: isoDay(monday), to: isoDay(monday.add(6, 'day')) };
}

export function EventCalendarPage({ title }: { title: string }) {
  const { t, i18n } = useTranslation();
  const { token } = theme.useToken();
  const navigate = useNavigate();
  const lang = i18n.language.startsWith('en') ? 'en' : 'vi';
  const today = utcToIct(new Date());
  const [cursor, setCursor] = useState<Dayjs>(dayjs(new Date(today.year, today.month - 1, today.day)));
  const [mode, setMode] = useState<'month' | 'week'>('month');
  const win = windowOf(cursor, mode);
  const q = useEventWindow(win.from, win.to);
  const def = CONFIG_TYPES.events;
  const eventsOn = (day: string) =>
    q.items.filter((e) => {
      const r = eventDays((e.payload.schedule ?? {}) as Schedule);
      return r && r.from <= day && day <= r.to;
    });
  const nameOfEvent = (e: GameConfigVersion) => e.title || nameOf(def, e.payload, lang) || String(e.payload.code ?? '');
  const bar = (e: GameConfigVersion) => (
    <li key={e.config_id}>
      <Button
        type="text"
        size="small"
        onClick={() => navigate(editorPathOf('events', e.config_id, e.version))}
        style={{ borderInlineStart: `${token.lineWidthBold * 2}px solid ${kindColor(e.payload.event_kind as string | undefined, token)}` }}
        data-testid={`calendar-event-${e.config_id}`}
        data-kind={String(e.payload.event_kind ?? '')}
      >
        {nameOfEvent(e)} {(e.calendar_warnings?.length ?? 0) > 0 ? <Tag color="warning">{t('gameOps.calendar.warningTag')}</Tag> : null}
      </Button>
    </li>
  );
  const weekDays = Array.from({ length: 7 }, (_, i) => dayjs(win.from).add(i, DAY));
  const warnings = q.items.flatMap((e) => (e.calendar_warnings ?? []).map((w) => ({ e, w })));

  return (
    <Flex vertical gap="middle" data-testid="event-calendar">
      <Typography.Title level={1}>{title}</Typography.Title>
      <Flex gap="small" align="center" wrap>
        <Segmented
          aria-label={t('gameOps.calendar.view')}
          value={mode}
          onChange={(v) => setMode(v as 'month' | 'week')}
          options={[
            { value: 'month', label: t('gameOps.calendar.month') },
            { value: 'week', label: t('gameOps.calendar.week') },
          ]}
        />
        <Button onClick={() => setCursor(cursor.subtract(1, mode))}>{t('gameOps.calendar.prev')}</Button>
        <Button onClick={() => setCursor(cursor.add(1, mode))}>{t('gameOps.calendar.next')}</Button>
        <Typography.Text strong data-testid="calendar-window">{t('gameOps.calendar.window', { from: win.from, to: win.to })}</Typography.Text>
        <Typography.Text type="secondary">{t('gameOps.calendar.ict')}</Typography.Text>
      </Flex>
      {q.error ? <GameOpsError error={q.error} /> : null}
      {warnings.length > 0 && (
        <Alert
          type="warning"
          showIcon
          data-testid="calendar-warnings"
          message={t('gameOps.calendar.warnings', { count: warnings.length })}
          description={
            <ul>
              {warnings.map(({ e, w }) => (
                <li key={`${e.config_id}:${w.other_config_id}:${w.code}`}>
                  {t(`gameOps.calendar.warning.${w.code}`, { event: nameOfEvent(e), other: w.other_title ?? w.other_config_id, area: w.area_id ?? '' })}
                </li>
              ))}
            </ul>
          }
        />
      )}
      {q.isPending ? (
        <Skeleton active />
      ) : mode === 'month' ? (
        <Calendar
          value={cursor}
          onPanelChange={(d) => setCursor(d)}
          onSelect={(d) => setCursor(d)}
          headerRender={() => null}
          cellRender={(d, info) => (info.type === 'date' ? <ul aria-label={isoDay(d)}>{eventsOn(isoDay(d)).map(bar)}</ul> : null)}
        />
      ) : (
        <Flex vertical gap="small" data-testid="calendar-week">
          {weekDays.map((d) => (
            <section key={isoDay(d)} aria-label={isoDay(d)}>
              <Typography.Text strong>{d.format(DAY_LABEL)}</Typography.Text>
              <ul>
                {eventsOn(isoDay(d)).map((e) => (
                  <Flex key={e.config_id} gap="small" align="center">
                    {bar(e)}
                    <DomainTag kind="content_status" value={e.status} />
                  </Flex>
                ))}
              </ul>
            </section>
          ))}
        </Flex>
      )}
    </Flex>
  );
}
