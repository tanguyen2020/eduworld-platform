// F122 T007 — ops-ui chat-safety pieces: highlight only on the text the user typed (F122 edge case, R15), SLA badge
// overdue = error status + text (F122-AC-01), "Luôn bật" lock (F122-AC-10/22). Placeholder text only (R15).
import { render, screen } from '@testing-library/react';
import { I18nextProvider } from 'react-i18next';
import { describe, expect, it } from 'vitest';
import { createTestI18n } from '../test/i18n';
import { AlwaysOnLock } from './AlwaysOnLock';
import { highlightParts, SensitiveTermTester } from './SensitiveTermTester';
import { SlaBadge } from './SlaBadge';

const i18n = () => createTestI18n('vi-VN');

describe('highlightParts', () => {
  it('splits the typed text into plain and matched parts; out-of-range and overlapping matches are clamped', () => {
    expect(highlightParts('ab zqx cd', [{ start: 3, end: 6 }])).toEqual([
      { text: 'ab ', hit: false },
      { text: 'zqx', hit: true },
      { text: ' cd', hit: false },
    ]);
    expect(highlightParts('zq', [{ start: -2, end: 99 }, { start: 0, end: 1 }])).toEqual([{ text: 'zq', hit: true }]);
    expect(highlightParts('', [{ start: 0, end: 3 }])).toEqual([]);
  });
});

describe('chat-safety components', () => {
  it('SensitiveTermTester marks only the typed text, shows the masked text and the categories', async () => {
    render(
      <I18nextProvider i18n={await i18n()}>
        <SensitiveTermTester
          maxLength={500}
          onTest={() => undefined}
          result={{ text: 'ban zqabc', outcome: { decision: 'mask', masked_text: 'ban *****', matches: [{ category_code: 'insult', severity: 'medium', action: 'mask', start: 4, end: 9 }] } }}
        />
      </I18nextProvider>,
    );
    expect(screen.getByText('zqabc').tagName).toBe('MARK');
    expect(screen.getByTestId('tester-masked')).toHaveTextContent('ban *****');
    expect(screen.getByLabelText('Chuỗi cần thử')).toHaveAttribute('maxlength', '500');
    expect(screen.getByText(/insult · mức medium · Che/)).toBeInTheDocument();
  });

  it('SlaBadge: overdue uses the error status and the text "Quá hạn"; done has no deadline state', async () => {
    const inst = await i18n();
    const now = new Date('2026-11-23T08:00:00Z');
    const { rerender } = render(
      <I18nextProvider i18n={inst}>
        <SlaBadge dueAt="2026-11-23T07:00:00Z" now={now} />
      </I18nextProvider>,
    );
    expect(screen.getByText(/^Quá hạn/).closest('[data-sla]')).toHaveAttribute('data-sla', 'overdue');
    rerender(
      <I18nextProvider i18n={inst}>
        <SlaBadge dueAt="2026-11-23T09:00:00Z" now={now} />
      </I18nextProvider>,
    );
    expect(screen.getByText(/^Hạn/).closest('[data-sla]')).toHaveAttribute('data-sla', 'pending');
    rerender(
      <I18nextProvider i18n={inst}>
        <SlaBadge dueAt="2026-11-23T07:00:00Z" now={now} done />
      </I18nextProvider>,
    );
    expect(screen.getByText('Đã xong')).toBeInTheDocument();
  });

  it('AlwaysOnLock shows "Luôn bật" and no switch', async () => {
    render(
      <I18nextProvider i18n={await i18n()}>
        <AlwaysOnLock reason="x" />
      </I18nextProvider>,
    );
    expect(screen.getByText('Luôn bật')).toBeInTheDocument();
    expect(screen.queryByRole('switch')).toBeNull();
  });
});
