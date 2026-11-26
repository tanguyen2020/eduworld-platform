// F108 T026 (US3) — F108-AC-16 (TC-CMS-TEN-03): a System Admin in workspace C must pick one target tenant before
// creating or acting: without one the create button is disabled and no challenge API is called; with one every call
// carries its X-Tenant-Id (and createChallenge its `tenant_id`); switching the tenant while the wizard is open resets
// the wizard and reloads the templates of the new tenant (edge case of the spec). Backend = MOCK (challenges-mock.ts).
import { act, cleanup, fireEvent, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { tenantFilterStore } from '../../../../shell/tenantFilterStore';
import type { RecordedRequest } from '../../../../test/portal-harness';
import { renderChallenges, stopPortals, TENANTS } from './challenges-mock';
import { fillWizard, next, pickOption } from './wizard-helpers';

afterEach(() => {
  stopPortals();
  cleanup();
  tenantFilterStore.set([]);
});

const challengeCalls = (b: { requests: RecordedRequest[] }) => b.requests.filter((r) => /^\/(lms\/challenges|lms\/catalog\/challenge-templates|competition\/)/.test(r.path));

describe('System Admin và tenant đích (F108-AC-16)', () => {
  it('no tenant picked: notice, create button disabled, no challenge call; wizard URL asks for a tenant too', async () => {
    const { backend } = await renderChallenges('/school-lms/challenges', { as: 'sa.admin', account: null });
    expect(await screen.findByText('Chọn đúng một tenant để làm việc')).toBeInTheDocument();
    expect(await screen.findByRole('button', { name: 'Tạo thử thách' })).toBeDisabled();
    expect(challengeCalls(backend)).toHaveLength(0);
    stopPortals();
    cleanup();
    const r2 = await renderChallenges('/school-lms/challenges/new', { as: 'sa.admin', account: null });
    expect(await screen.findByText('Chọn đúng một tenant để làm việc')).toBeInTheDocument();
    expect(screen.queryByRole('combobox', { name: 'Template' })).toBeNull();
    expect(challengeCalls(r2.backend)).toHaveLength(0);
  });

  it('tenant picked: list and create carry X-Tenant-Id of that tenant and createChallenge its tenant_id', async () => {
    const { backend } = await renderChallenges('/school-lms/challenges', { as: 'sa.admin', account: null });
    await screen.findByText('Chọn đúng một tenant để làm việc');
    act(() => tenantFilterStore.set([TENANTS.thDemoA.tenant_id]));
    const create = await screen.findByRole('button', { name: 'Tạo thử thách' });
    await waitFor(() => expect(create).not.toBeDisabled());
    expect(await screen.findByText('Giải Toán khối 3')).toBeInTheDocument();
    fireEvent.click(create);
    await fillWizard({ include: false, exclude: false });
    fireEvent.click(screen.getByRole('button', { name: 'Tạo thử thách' }));
    expect(await screen.findByTestId('challenge-detail')).toBeInTheDocument();
    const post = backend.calls('POST', '/lms/challenges')[0]!;
    expect(post.headers['x-tenant-id']).toBe(TENANTS.thDemoA.tenant_id);
    expect((post.body as Record<string, unknown>).tenant_id).toBe(TENANTS.thDemoA.tenant_id);
    const calls = challengeCalls(backend);
    expect(calls.every((r) => r.headers['x-tenant-id'] === TENANTS.thDemoA.tenant_id)).toBe(true);
  });

  it('switching the tenant while the wizard is open resets it and reloads the templates of the new tenant', async () => {
    tenantFilterStore.set([TENANTS.thDemoA.tenant_id]);
    const { backend } = await renderChallenges('/school-lms/challenges/new', { as: 'sa.admin', account: null });
    await pickOption('Template', 'Giải Toán khối');
    next();
    fireEvent.change(await screen.findByLabelText('Tên thử thách'), { target: { value: 'Bản nháp tenant A' } });
    act(() => tenantFilterStore.set([TENANTS.thcsDemoB.tenant_id]));
    // back to the first step, nothing kept from tenant A
    const template = await screen.findByRole('combobox', { name: 'Template' });
    expect(screen.queryByDisplayValue('Bản nháp tenant A')).toBeNull();
    expect(screen.queryByText('Giải Toán khối')).toBeNull();
    fireEvent.mouseDown(template);
    expect(await screen.findByTitle('Thử thách THCS B')).toBeInTheDocument();
    expect(screen.queryByTitle('Giải Toán khối')).toBeNull();
    const b = backend.calls('GET', '/lms/catalog/challenge-templates').filter((r) => r.headers['x-tenant-id'] === TENANTS.thcsDemoB.tenant_id);
    expect(b.length).toBeGreaterThan(0);
  });
});
