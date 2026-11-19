// F050-AC-05 (TC-UI-005): OpsThemeProvider applies the generated light/dark/compact theme, Ant Design locale
// vi_VN / en_US and the matching dayjs locale, without CSS overrides of Ant Design internals.
import { render, screen } from '@testing-library/react';
import { ConfigProvider, DatePicker, theme } from 'antd';
import dayjs from 'dayjs';
import fs from 'node:fs';
import path from 'node:path';
import { useContext } from 'react';
import { describe, expect, it } from 'vitest';
import { compactOverrides, compactTheme, darkTheme, lightTheme } from './antd-theme';
import { OpsThemeProvider, resolveThemeConfig, useOpsTheme } from './OpsThemeProvider';

function Probe() {
  const { token } = theme.useToken();
  const { locale } = useContext(ConfigProvider.ConfigContext);
  const ops = useOpsTheme();
  return (
    <output data-testid="probe">
      {JSON.stringify({ colorPrimary: token.colorPrimary, fontSize: token.fontSize, controlHeight: token.controlHeight, colorBgBase: token.colorBgBase, fontFamily: token.fontFamily, locale: locale?.locale, mode: ops.mode })}
    </output>
  );
}

const probe = () => JSON.parse(screen.getByTestId('probe').textContent ?? '{}');

describe('OpsThemeProvider', () => {
  it('light (default) uses lightTheme seeds and Vietnamese', () => {
    render(<OpsThemeProvider><Probe /></OpsThemeProvider>);
    const p = probe();
    expect(p.colorPrimary.toUpperCase()).toBe(lightTheme.token?.colorPrimary);
    expect(p.fontSize).toBe(lightTheme.token?.fontSize);
    expect(p.fontFamily).toBe(lightTheme.token?.fontFamily);
    expect(p.locale).toBe('vi');
    expect(dayjs.locale()).toBe('vi');
  });

  it('dark uses darkTheme seeds and exposes data-theme for --ew-* variables', () => {
    const { container } = render(<OpsThemeProvider mode="dark"><Probe /></OpsThemeProvider>);
    const p = probe();
    // darkAlgorithm derives map tokens from the generated seeds
    const expected = theme.getDesignToken(darkTheme);
    expect(p.colorPrimary).toBe(expected.colorPrimary);
    expect(p.colorBgBase.toUpperCase()).toBe(darkTheme.token?.colorBgBase);
    expect(p.colorPrimary).not.toBe(theme.getDesignToken(lightTheme).colorPrimary);
    expect(p.mode).toBe('dark');
    expect(container.querySelector('[data-theme="dark"]')).not.toBeNull();
  });

  it('compact uses the compact seeds (13px, control 28) and never a font below 13px', () => {
    render(<OpsThemeProvider density="compact"><Probe /></OpsThemeProvider>);
    const p = probe();
    expect(p.fontSize).toBe(compactTheme.token?.fontSize);
    expect(p.fontSize).toBeGreaterThanOrEqual(13);
    expect(p.controlHeight).toBe(compactTheme.token?.controlHeight);
  });

  it('dark + compact merges compactOverrides into the dark seeds', () => {
    const cfg = resolveThemeConfig('dark', 'compact');
    expect(cfg.algorithm).toBe(darkTheme.algorithm);
    expect(cfg.token).toMatchObject({ ...darkTheme.token, ...compactOverrides });
  });

  it('en-US switches Ant Design and dayjs locales', () => {
    render(
      <OpsThemeProvider locale="en-US">
        <Probe />
        <DatePicker open={false} />
      </OpsThemeProvider>,
    );
    expect(probe().locale).toBe('en');
    expect(dayjs.locale()).toBe('en');
  });

  it('ops-ui CSS never targets Ant Design internal classes (.ant-*)', () => {
    const dir = path.resolve(__dirname, '..');
    const css: string[] = [];
    const walk = (d: string) => {
      for (const e of fs.readdirSync(d, { withFileTypes: true })) {
        const p = path.join(d, e.name);
        if (e.isDirectory()) walk(p);
        else if (/\.(css|tsx?)$/.test(e.name) && !e.name.endsWith('.test.tsx')) css.push(fs.readFileSync(p, 'utf8'));
      }
    };
    walk(dir);
    expect(css.join('\n')).not.toMatch(/\.ant-[a-z]/);
  });
});
