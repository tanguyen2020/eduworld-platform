// TagListInput (F082-AC-05, AC-19): the tags of one language of a GIF — an input capped by the schema `maxLength` with a
// counter, checked against the schema `pattern` (lowercase, no diacritics) with the reason shown, at most `maxItems` tags;
// the server checks them again (VALIDATION_FAILED). Display only.
import { Button, Flex, Form, Input, Tag, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

export interface TagListInputProps {
  id: string;
  label: string;
  addLabel: string;
  value: string[];
  onChange: (tags: string[]) => void;
  maxLength: number;
  maxItems: number;
  pattern?: string;
  readOnly?: boolean;
  error?: string;
}

export function TagListInput({ id, label, addLabel, value, onChange, maxLength, maxItems, pattern, readOnly = false, error }: TagListInputProps) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState('');
  const text = draft.trim();
  const invalid = text !== '' && !!pattern && !new RegExp(pattern).test(text);
  const full = value.length >= maxItems;
  const add = () => {
    if (!text || invalid || full || value.includes(text)) return;
    onChange([...value, text]);
    setDraft('');
  };
  return (
    <Form.Item
      label={label}
      htmlFor={id}
      validateStatus={invalid || error ? 'error' : undefined}
      help={invalid ? t('gameOps.chatAssets.gif.tagPattern') : error}
      extra={t('gameOps.chatAssets.gif.tagCount', { count: value.length, max: maxItems })}
    >
      <Flex vertical gap="small">
        {!readOnly && (
          <Flex gap="small">
            <Input id={id} aria-label={label} maxLength={maxLength} showCount value={draft} onChange={(e) => setDraft(e.target.value)} onPressEnter={add} />
            <Button aria-label={addLabel} disabled={!text || invalid || full} onClick={add}>
              {t('gameOps.chatAssets.gif.addTag')}
            </Button>
          </Flex>
        )}
        <Flex gap="small" wrap>
          {value.length === 0 && <Typography.Text type="secondary">{t('gameOps.chatAssets.gif.noTags')}</Typography.Text>}
          {value.map((tag) => (
            <Tag key={tag} closable={!readOnly} onClose={() => onChange(value.filter((x) => x !== tag))}>
              {tag}
            </Tag>
          ))}
        </Flex>
      </Flex>
    </Form.Item>
  );
}
