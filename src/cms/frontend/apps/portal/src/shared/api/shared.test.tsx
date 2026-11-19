// F051 T050 — utilities shared by F052–F057: one Idempotency-Key per user intent (same data → same key, changed data
// → new key, success → forgotten), ReasonDialog (reason required, maxLength + counter), 422 VALIDATION_FAILED →
// form field errors.
import { OpsThemeProvider } from '@eduworld/ops-ui';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { I18nextProvider } from 'react-i18next';
import { describe, expect, it, vi } from 'vitest';
import { createI18n } from '../../i18n';
import { ReasonDialog } from '../../shell/ReasonDialog';
import { mapValidationErrors } from './mapValidationErrors';
import { IdempotencyKeys } from './middleware/idempotency';
import { ApiError } from './middleware/problem';
import { fingerprint, IntentTracker } from './useIdempotentMutation';

describe('IntentTracker + IdempotencyKeys', () => {
  it('same data → same intent → same key; other data → new key; done → new key', () => {
    let n = 0;
    const tracker = new IntentTracker(() => `intent-${++n}`);
    const keys = new IdempotencyKeys();
    const k1 = keys.forIntent(tracker.intentFor({ b: 2, a: 1 }));
    expect(keys.forIntent(tracker.intentFor({ a: 1, b: 2 }))).toBe(k1);
    const k2 = keys.forIntent(tracker.intentFor({ a: 1, b: 3 }));
    expect(k2).not.toBe(k1);
    tracker.done();
    expect(tracker.intentFor({ a: 1, b: 3 })).toBe('intent-3');
  });

  it('fingerprint ignores key order', () => {
    expect(fingerprint({ a: 1, b: { d: 1, c: 2 } })).toBe(fingerprint({ b: { c: 2, d: 1 }, a: 1 }));
  });
});

describe('mapValidationErrors', () => {
  it('maps errors[].field to known form fields, keeps the rest apart', () => {
    const e = new ApiError({ status: 422, code: 'VALIDATION_FAILED', i18nKey: 'error.VALIDATION_FAILED', fieldErrors: { name: { code: 'TOO_LONG' }, 'items.0.title': { code: 'REQUIRED' }, other: { code: 'X' } } });
    const r = mapValidationErrors(e, ['name', 'items'], (k) => `t:${k}`, () => false);
    expect(r.fields).toEqual([
      { name: ['name'], errors: ['t:error.VALIDATION_FAILED'] },
      { name: ['items', 0, 'title'], errors: ['t:error.VALIDATION_FAILED'] },
    ]);
    expect(r.unknown).toEqual(['other']);
    expect(mapValidationErrors(new ApiError({ status: 409, code: 'CONFLICT', i18nKey: 'x' }), [], (k) => k, () => true).fields).toEqual([]);
  });
});

describe('ReasonDialog', () => {
  it('requires a reason and caps it at the OpenAPI maxLength', async () => {
    const i18n = await createI18n('vi-VN');
    const onSubmit = vi.fn();
    render(
      <I18nextProvider i18n={i18n}>
        <OpsThemeProvider>
          <ReasonDialog open title="Tắt menu" maxLength={10} onSubmit={onSubmit} onCancel={() => undefined} />
        </OpsThemeProvider>
      </I18nextProvider>,
    );
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: 'Xác nhận' }));
    expect(await screen.findByText('Vui lòng nhập lý do.')).toBeInTheDocument();
    const box = screen.getByLabelText('Lý do');
    await user.type(box, 'mot hai ba bon nam');
    expect((box as HTMLTextAreaElement).value).toHaveLength(10);
    await user.click(screen.getByRole('button', { name: 'Xác nhận' }));
    await vi.waitFor(() => expect(onSubmit).toHaveBeenCalledWith('mot hai ba'), { timeout: 10_000 });
  });
});
