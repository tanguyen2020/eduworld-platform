// Owner 2026-11-23 (CODE_PLAN §4.3, ADMIN_PORTAL §1.6): the header tenant switch is a SearchSelect — search ignores case
// and Vietnamese diacritics, and an unknown text shows the empty state instead of an empty list.
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { I18nextProvider } from 'react-i18next';
import { describe, expect, it, vi } from 'vitest';
import { createI18n } from '../i18n';
import { TenantSwitcherView } from './TenantControls';

const TENANTS = [
  { tenant_id: '0193a4d2-0000-7000-8000-0000000000a1', name: 'Trường Tiểu học Đà Nẵng', code: 'th-dn' },
  { tenant_id: '0193a4d2-0000-7000-8000-0000000000a2', name: 'THCS Demo', code: 'thcs-demo' },
];

describe('TenantSwitcherView (SearchSelect)', () => {
  it('"da nang" finds "Đà Nẵng" without diacritics; choosing it calls onChange', async () => {
    const i18n = await createI18n('vi-VN');
    const onChange = vi.fn();
    const user = userEvent.setup({ delay: null });
    render(
      <I18nextProvider i18n={i18n}>
        <TenantSwitcherView tenants={TENANTS} value={TENANTS[1]!.tenant_id} onChange={onChange} />
      </I18nextProvider>,
    );
    const box = screen.getByRole('combobox', { name: 'Đơn vị làm việc' });
    await user.click(box);
    await user.type(box, 'da nang');
    const option = await screen.findByTitle('Trường Tiểu học Đà Nẵng (th-dn)');
    // the other tenant stays only in the selection box, not in the filtered list
    expect(screen.getAllByTitle('THCS Demo (thcs-demo)').filter((el) => el.closest('.ant-select-dropdown'))).toHaveLength(0);
    await user.click(option);
    await waitFor(() => expect(onChange).toHaveBeenCalledWith(TENANTS[0]!.tenant_id, expect.anything()));
  });
});
