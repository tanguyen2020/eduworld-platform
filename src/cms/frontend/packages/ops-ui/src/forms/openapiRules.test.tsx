// F050-AC-09 (TC-TEN-024 client side): rules from an OpenAPI schema (required, maxLength, pattern), counter,
// typing beyond maxLength blocked, error shown at the field.
import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Form, Input, type FormInstance } from 'antd';
import { createRef } from 'react';
import { I18nextProvider } from 'react-i18next';
import { describe, expect, it } from 'vitest';
import { createTestI18n } from '../test/i18n';
import { OpsThemeProvider } from '../theme/OpsThemeProvider';
import { openapiInputProps } from './CharCounter';
import { openapiRules, type OpenapiStringSchema } from './openapiRules';

const schema: OpenapiStringSchema = { type: 'string', maxLength: 5, pattern: '^[a-z0-9]+$' };

describe('openapiRules', () => {
  it('maps required, maxLength, minLength and pattern', async () => {
    const i18n = await createTestI18n();
    const rules = openapiRules({ ...schema, minLength: 2 }, { required: true, t: i18n.t });
    expect(rules).toEqual([
      { required: true, whitespace: true, message: 'Trường này bắt buộc.' },
      { type: 'string', max: 5, message: 'Tối đa 5 ký tự.' },
      { type: 'string', min: 2, message: 'Tối thiểu 2 ký tự.' },
      { pattern: /^[a-z0-9]+$/u, message: 'Giá trị không đúng định dạng.' },
    ]);
  });

  it('no constraint → no rule', async () => {
    const i18n = await createTestI18n();
    expect(openapiRules({ type: 'string' }, { t: i18n.t })).toEqual([]);
  });

  it('blocks typing beyond maxLength, shows the counter and reports errors at the field', async () => {
    const i18n = await createTestI18n();
    const form = createRef<FormInstance>();
    render(
      <I18nextProvider i18n={i18n}>
        <OpsThemeProvider>
          <Form ref={form}>
            <Form.Item name="code" label="code" rules={openapiRules(schema, { required: true, t: i18n.t })}>
              <Input {...openapiInputProps(schema)} />
            </Form.Item>
          </Form>
        </OpsThemeProvider>
      </I18nextProvider>,
    );
    const input = screen.getByRole('textbox');
    await userEvent.type(input, 'abcdefgh');
    expect(input).toHaveValue('abcde');
    expect(screen.getByText('5/5')).toBeInTheDocument();

    // a value loaded from elsewhere that exceeds maxLength is rejected at the field
    await act(async () => {
      form.current!.setFieldsValue({ code: 'abcdefg' });
      await form.current!.validateFields().catch(() => undefined);
    });
    await waitFor(() => expect(screen.getByText('Tối đa 5 ký tự.')).toBeInTheDocument());

    await act(async () => {
      form.current!.setFieldsValue({ code: 'AB' });
      await form.current!.validateFields().catch(() => undefined);
    });
    await waitFor(() => expect(screen.getByText('Giá trị không đúng định dạng.')).toBeInTheDocument());
  });
});
