// TranslationExportDialog (F082-AC-15, ADMIN_PORTAL §1.5): export the game texts to translate — config types (game module
// only), optional items (≤ `maxItems` of TranslationExportRequest.content_ids), target language, XLIFF 2.1 or CSV, only the
// missing/outdated fields (default on). The caller passes every option (all pages, searchable) and sends the request with
// its Idempotency-Key. Presentational: no API call, no machine translation.
import { Form, Modal, Radio, Switch } from 'antd';
import { useEffect, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { OPS_UI_NS } from '../i18n';
import { SearchSelect } from '../select/SearchSelect';

export interface TranslationExportValues {
  content_types: string[];
  content_ids: string[];
  source_language: 'vi' | 'en';
  target_language: 'vi' | 'en';
  format: 'xliff' | 'csv';
  only_missing: boolean;
}

export interface TranslationExportDialogProps {
  open: boolean;
  typeOptions: { value: string; label: string }[];
  /** Items of the chosen types (every page). */
  itemOptions: { value: string; label: string }[];
  itemsLoading?: boolean;
  itemsTruncated?: boolean;
  /** maxItems of content_ids (contract). */
  maxItems: number;
  /** maxItems of content_types (contract); absent = no cap shown. */
  maxTypes?: number;
  onTypesChange?: (types: string[]) => void;
  submitting?: boolean;
  /** Problem of the last attempt (code + trace id), rendered by the caller. */
  error?: ReactNode;
  onCancel: () => void;
  onSubmit: (values: TranslationExportValues) => void;
}

const INITIAL: TranslationExportValues = { content_types: [], content_ids: [], source_language: 'vi', target_language: 'en', format: 'xliff', only_missing: true };

export function TranslationExportDialog(p: TranslationExportDialogProps) {
  const { t } = useTranslation(OPS_UI_NS);
  const [form] = Form.useForm<TranslationExportValues>();
  useEffect(() => {
    if (p.open) form.setFieldsValue(INITIAL);
  }, [p.open, form]);
  const submit = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    const target = v.target_language;
    p.onSubmit({
      content_types: v.content_types ?? [],
      content_ids: v.content_ids ?? [],
      source_language: target === 'en' ? 'vi' : 'en',
      target_language: target,
      format: v.format,
      only_missing: v.only_missing,
    });
  };
  return (
    <Modal open={p.open} title={t('translations.export.title')} okText={t('translations.export.submit')} cancelText={t('translations.cancel')} onOk={() => void submit()} onCancel={p.onCancel} confirmLoading={p.submitting} destroyOnHidden>
      <Form form={form} layout="vertical" initialValues={INITIAL} component="div">
        <Form.Item name="content_types" label={t('translations.export.types')} htmlFor="tr-export-types" extra={t('translations.export.typesHint')}>
          <SearchSelect id="tr-export-types" aria-label={t('translations.export.types')} mode="multiple" options={p.typeOptions} maxCount={p.maxTypes} onChange={(v) => p.onTypesChange?.(v as string[])} />
        </Form.Item>
        <Form.Item name="content_ids" label={t('translations.export.items', { max: p.maxItems })} htmlFor="tr-export-items" extra={t('translations.export.itemsHint')}>
          <SearchSelect
            id="tr-export-items"
            aria-label={t('translations.export.items', { max: p.maxItems })}
            mode="multiple"
            options={p.itemOptions}
            loading={p.itemsLoading}
            truncated={p.itemsTruncated}
            maxCount={p.maxItems}
          />
        </Form.Item>
        <Form.Item name="target_language" label={t('translations.target')}>
          <Radio.Group
            optionType="button"
            options={[
              { value: 'en', label: t('translations.lang.en') },
              { value: 'vi', label: t('translations.lang.vi') },
            ]}
          />
        </Form.Item>
        <Form.Item name="format" label={t('translations.format')}>
          <Radio.Group
            optionType="button"
            options={[
              { value: 'xliff', label: 'XLIFF 2.1' },
              { value: 'csv', label: 'CSV' },
            ]}
          />
        </Form.Item>
        <Form.Item name="only_missing" valuePropName="checked" label={t('translations.export.onlyMissing')} htmlFor="tr-export-only-missing">
          <Switch id="tr-export-only-missing" aria-label={t('translations.export.onlyMissing')} />
        </Form.Item>
      </Form>
      {p.error}
    </Modal>
  );
}
