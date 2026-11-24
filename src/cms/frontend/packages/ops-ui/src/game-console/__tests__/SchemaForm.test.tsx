// F079 T012, T030, T037 — SchemaForm (ops-ui): the form of a game config is built from the JSON Schema of its type
// (F079-AC-02): required marker, maxLength cap + counter (ADR-0041, AC-20), enum → searchable select, `const` fields set
// and never shown (schema_version, paid_track — AC-15), server-managed keys hidden, bilingual vi/en text with its status,
// field errors placed by path whatever the separator the server used (AC-05), read-only mode.
import { fireEvent, screen, within } from '@testing-library/react';
import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { renderOps } from '../../test/render';
import { SchemaForm } from '../SchemaForm';
import { canonicalPath, initialValue, setIn, type JsonSchema } from '../schemaModel';

const schema: JsonSchema = {
  type: 'object',
  required: ['schema_version', 'code', 'name', 'kind', 'paid_track', 'tiers'],
  properties: {
    schema_version: { const: 1 },
    season_track_id: { type: 'string', maxLength: 64 },
    code: { type: 'string', maxLength: 12 },
    name: { $ref: '#/$defs/localized_text' },
    kind: { enum: ['festival', 'weekend'] },
    days: { type: 'integer', minimum: 1, maximum: 30 },
    paid_track: { const: false },
    featured: { type: 'boolean' },
    tiers: { type: 'array', items: { type: 'object', required: ['tier'], properties: { tier: { type: 'integer', minimum: 1 } } } },
  },
  $defs: {
    localized_text: {
      type: 'object',
      required: ['vi', 'en'],
      additionalProperties: false,
      properties: { vi: { type: 'string', minLength: 1, maxLength: 40 }, en: { type: 'string', minLength: 1, maxLength: 40 } },
    },
  },
};

function Harness({ errors = {}, readOnly = false, onValue }: { errors?: Record<string, string>; readOnly?: boolean; onValue?: (v: unknown) => void }) {
  const [value, setValue] = useState<Record<string, unknown>>(() => initialValue(schema) as Record<string, unknown>);
  return (
    <SchemaForm
      schema={schema}
      value={value}
      managedKeys={['season_track_id']}
      errors={errors}
      readOnly={readOnly}
      label={(key) => `L:${key}`}
      onChange={(v) => {
        setValue(v as Record<string, unknown>);
        onValue?.(v);
      }}
    />
  );
}

describe('schemaModel', () => {
  it('canonicalPath accepts dotted, slashed, JSON pointer and bracket paths, and drops a leading payload segment', () => {
    expect(canonicalPath('steps.0.area_id')).toBe('steps/0/area_id');
    expect(canonicalPath('/steps/0/area_id')).toBe('steps/0/area_id');
    expect(canonicalPath('payload/steps[0].area_id')).toBe('steps/0/area_id');
    expect(canonicalPath('caps/xp_per_day')).toBe('caps/xp_per_day');
  });

  it('initialValue sets the const fields only (schema_version, paid_track = false)', () => {
    expect(initialValue(schema)).toEqual({ schema_version: 1, paid_track: false });
  });

  it('setIn writes a nested path immutably and creates arrays for numeric segments', () => {
    const a = { x: { y: 1 } };
    const b = setIn(a, ['list', 0, 'v'], 2) as Record<string, unknown>;
    expect(b).toEqual({ x: { y: 1 }, list: [{ v: 2 }] });
    expect(a).toEqual({ x: { y: 1 } });
  });
});

describe('SchemaForm (F079-AC-02, AC-15, AC-20)', () => {
  it('renders fields from the schema: required marker, maxLength cap with counter, enum select, bilingual text; no const nor managed control', async () => {
    const onValue = vi.fn();
    await renderOps(<Harness onValue={onValue} />);
    const code = screen.getByLabelText('L:code');
    expect(code).toHaveAttribute('maxlength', '12');
    expect(screen.getByText('0/12')).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: 'L:kind' })).toBeInTheDocument();
    // const fields and the server-managed id have no control
    expect(screen.queryByLabelText('L:schema_version')).toBeNull();
    expect(screen.queryByText('L:paid_track')).toBeNull();
    expect(screen.queryByLabelText('L:season_track_id')).toBeNull();
    // bilingual: both languages side by side
    expect(screen.getByTestId('field-name')).toHaveTextContent('VI');
    expect(screen.getByTestId('field-name')).toHaveTextContent('EN');
    fireEvent.change(code, { target: { value: 'tet2027' } });
    expect(onValue).toHaveBeenLastCalledWith({ schema_version: 1, paid_track: false, code: 'tet2027' });
    expect(screen.getByText('7/12')).toBeInTheDocument();
  });

  it('adds and removes array items; numbers keep their min/max', async () => {
    const onValue = vi.fn();
    await renderOps(<Harness onValue={onValue} />);
    const tiers = screen.getByRole('group', { name: 'L:tiers' });
    fireEvent.click(within(tiers).getByRole('button', { name: /Thêm/ }));
    expect(onValue).toHaveBeenLastCalledWith(expect.objectContaining({ tiers: [{}] }));
    const tier = await within(tiers).findByRole('spinbutton', { name: 'L:tier' });
    expect(tier).toHaveAttribute('aria-valuemin', '1');
    fireEvent.click(within(tiers).getByRole('button', { name: /Xóa mục 1/ }));
    expect(onValue).toHaveBeenLastCalledWith(expect.objectContaining({ tiers: [] }));
  });

  it('shows server field errors on the field of the path (AC-05)', async () => {
    await renderOps(<Harness errors={{ code: 'Mã đã tồn tại', 'name/en': 'Thiếu bản tiếng Anh' }} />);
    expect(screen.getByText('Mã đã tồn tại')).toBeInTheDocument();
    expect(screen.getByText('Thiếu bản tiếng Anh')).toBeInTheDocument();
  });

  it('read-only: every control is disabled (published version)', async () => {
    await renderOps(<Harness readOnly />);
    expect(screen.getByLabelText('L:code')).toBeDisabled();
    expect(screen.queryByRole('button', { name: /Thêm/ })).toBeNull();
  });
});
