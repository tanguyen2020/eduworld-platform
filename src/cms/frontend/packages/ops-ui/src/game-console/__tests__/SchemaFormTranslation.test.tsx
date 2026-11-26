// F082 T019 — game texts in the generated form (F082-AC-10, ADMIN_PORTAL §1.5): every `localized_text` is two columns with
// `vi` first labelled "Gốc" (tabs VI | EN on a narrow screen), the field status comes from the server (`outdated_fields`
// → "Lỗi thời") and an outdated field offers "Xem thay đổi bản gốc" (the caller opens the diff). No machine translation.
import { fireEvent, screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { renderOps } from '../../test/render';
import { SchemaForm } from '../SchemaForm';
import type { JsonSchema } from '../schemaModel';

const schema: JsonSchema = {
  type: 'object',
  properties: {
    name: { $ref: '#/$defs/localized_text' },
    steps: { type: 'array', items: { type: 'object', properties: { title: { $ref: '#/$defs/localized_text' } } } },
  },
  $defs: {
    localized_text: { type: 'object', required: ['vi', 'en'], properties: { vi: { type: 'string', maxLength: 40 }, en: { type: 'string', maxLength: 40 } } },
  },
};

const value = { name: { vi: 'Rừng số', en: 'Number forest' }, steps: [{ title: { vi: 'Bước một', en: 'Step one' } }] };

describe('SchemaForm — game texts (F082-AC-10)', () => {
  it('F082-AC-10 vi column first with "Gốc", en second; no machine translation button', async () => {
    await renderOps(<SchemaForm schema={schema} value={value} onChange={vi.fn()} label={(k) => k} layout="columns" />);
    const field = screen.getByTestId('field-name');
    const labels = within(field).getAllByText(/^(VI|EN)$/).map((e) => e.textContent);
    expect(labels).toEqual(['VI', 'EN']);
    expect(within(field).getByText('Gốc')).toBeInTheDocument();
    expect(screen.queryByText(/dịch máy|tự động dịch|machine/i)).toBeNull();
  });

  it('F082-AC-10 an outdated field shows "Lỗi thời" and opens the diff of its pointer', async () => {
    const onShowDiff = vi.fn();
    await renderOps(<SchemaForm schema={schema} value={value} onChange={vi.fn()} label={(k) => k} translation={{ outdated: ['/steps/0/title'], onShowDiff }} />);
    const step = screen.getByTestId('field-steps-0-title');
    expect(within(step).getByText('Lỗi thời')).toBeInTheDocument();
    expect(within(screen.getByTestId('field-name')).getByText('Đủ')).toBeInTheDocument();
    fireEvent.click(within(step).getByRole('button', { name: 'Xem thay đổi bản gốc' }));
    expect(onShowDiff).toHaveBeenCalledWith('/steps/0/title');
  });

  it('F082-AC-10 narrow screens: tabs VI | EN', async () => {
    await renderOps(<SchemaForm schema={schema} value={value} onChange={vi.fn()} label={(k) => k} layout="tabs" />);
    const field = screen.getByTestId('field-name');
    expect(within(field).getAllByRole('tab').map((e) => e.textContent)).toEqual(['VI', 'EN']);
  });
});
