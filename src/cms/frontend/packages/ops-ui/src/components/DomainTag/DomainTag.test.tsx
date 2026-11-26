// F050-AC-08 (TC-UI-008): DomainTag uses domainColors from shared-tokens (per mode) and always shows a label.
import { render, screen } from '@testing-library/react';
import { I18nextProvider } from 'react-i18next';
import { describe, expect, it } from 'vitest';
import { domainColors } from '../../theme/antd-theme';
import { OpsThemeProvider } from '../../theme/OpsThemeProvider';
import { createTestI18n } from '../../test/i18n';
import { DomainTag, type DomainKind } from './DomainTag';

async function renderTag(kind: DomainKind, value: string, mode: 'light' | 'dark' = 'light', lng: 'vi-VN' | 'en-US' = 'vi-VN') {
  const i18n = await createTestI18n(lng);
  return render(
    <I18nextProvider i18n={i18n}>
      <OpsThemeProvider mode={mode}>
        <DomainTag kind={kind} value={value} />
      </OpsThemeProvider>
    </I18nextProvider>,
  );
}

const rgb = (hex: string) => {
  const n = parseInt(hex.slice(1), 16);
  return `rgb(${(n >> 16) & 255}, ${(n >> 8) & 255}, ${n & 255})`;
};

describe('DomainTag', () => {
  it('subject math: light color from domainColors + Vietnamese label', async () => {
    await renderTag('subject', 'math');
    expect(screen.getByText('Toán')).toBeInTheDocument();
    const dot = screen.getByTestId('domain-tag-dot');
    expect(dot).toHaveAttribute('aria-hidden', 'true');
    expect(dot.style.backgroundColor).toBe(rgb(domainColors['color.subject.math']!.light));
  });

  it('uses the dark variant in dark mode', async () => {
    await renderTag('rarity', 'epic', 'dark');
    expect(screen.getByText('Sử thi')).toBeInTheDocument();
    expect(screen.getByTestId('domain-tag-dot').style.backgroundColor).toBe(rgb(domainColors['color.rarity.epic']!.dark));
  });

  it('every content status has a label in vi and en (color is never the only channel)', async () => {
    for (const status of ['draft', 'in_review', 'approved', 'published', 'archived', 'rejected']) {
      expect(domainColors[`color.content_status.${status}`]).toBeDefined();
      const vi = await renderTag('content_status', status);
      const viText = vi.container.textContent ?? '';
      vi.unmount();
      const en = await renderTag('content_status', status, 'light', 'en-US');
      const enText = en.container.textContent ?? '';
      en.unmount();
      expect(viText.length).toBeGreaterThan(0);
      expect(enText.length).toBeGreaterThan(0);
      expect(viText).not.toBe(status);
    }
  });

  it('subject history (F094): reddish-brown shared colour + Vietnamese label', async () => {
    await renderTag('subject', 'history');
    expect(screen.getByText('Lịch sử & Địa lý')).toBeInTheDocument();
    expect(screen.getByTestId('domain-tag-dot').style.backgroundColor).toBe(rgb(domainColors['color.subject.history']!.light));
  });

  it('an unknown code still renders its text without a color dot', async () => {
    await renderTag('subject', 'music');
    expect(screen.getByText('music')).toBeInTheDocument();
    expect(screen.queryByTestId('domain-tag-dot')).toBeNull();
  });
});
