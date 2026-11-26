// GenericSchemaBuilder (F079 T017, T022, F079-AC-08): the form of a config type generated from its JSON Schema
// (reward rules, items, shop items, collections, world areas, story chapters, mini-game configs, achievements,
// challenges…) with reference pickers and ICT date-times. Prices are whatever the schema allows — the shop schema only has
// in-game currencies (`price.currency_code`); there is no real-money field anywhere (AGENTS R19). A type without schema
// gets the JSON editor validated by the server (spec edge case), said so in the UI.
import { SchemaForm, type JsonSchema } from '@eduworld/ops-ui/game-console';
import { Alert, Flex } from 'antd';
import { useTranslation } from 'react-i18next';
import { gameOverrides } from './fields';
import type { BuilderProps } from './types';
import { useFieldLabel } from './labels';

const FREE_FORM: JsonSchema = { type: 'object', properties: {} };

export function GenericSchemaBuilder({ def, value, onChange, readOnly, errors, omitKeys = [], translation }: BuilderProps & { omitKeys?: readonly string[] }) {
  const { t } = useTranslation();
  const label = useFieldLabel();
  if (!def.schema) {
    return (
      <Flex vertical gap="small">
        <Alert type="info" showIcon message={t('gameOps.editor.noSchema')} />
        <SchemaForm schema={{ type: 'object', properties: { payload: FREE_FORM } }} value={{ payload: value }} onChange={(v) => onChange(((v as { payload?: Record<string, unknown> }).payload ?? {}) as Record<string, unknown>)} label={label} readOnly={readOnly} errors={errors} />
      </Flex>
    );
  }
  return (
    <SchemaForm
      schema={def.schema}
      value={value}
      onChange={(v) => onChange(v as Record<string, unknown>)}
      label={label}
      managedKeys={def.managedKeys}
      omitKeys={omitKeys}
      errors={errors}
      readOnly={readOnly}
      renderOverride={gameOverrides(def.idField)}
      idPrefix={`gf-${def.type}`}
      translation={translation}
    />
  );
}
