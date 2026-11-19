// Small presentational pieces shared by workspace C screens: page title (h1), "SIS" source tag (ADR-0060), status tags,
// form rules bound to the ops-ui translator (F050-AC-09), and a notice of the write result. No data logic.
import { OPS_UI_NS, openapiInputProps, openapiRules, type OpenapiStringSchema } from '@eduworld/ops-ui';
import { Flex, Tag, Typography } from 'antd';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

export function PageTitle({ title, extra }: { title: string; extra?: ReactNode }) {
  return (
    <Flex justify="space-between" align="center" wrap="wrap" gap="small">
      <Typography.Title level={1} id="ew-menu-title">
        {title}
      </Typography.Title>
      {extra}
    </Flex>
  );
}

/** Data owned by SIS/CSV (read-only in the portal, ADR-0060). */
export function SourceTag({ source }: { source?: string | null }) {
  const { t } = useTranslation();
  if (source !== 'sis' && source !== 'import') return null;
  return <Tag data-testid="sis-tag">{t('portal.school_lms.common.sisTag')}</Tag>;
}

export function StatusTag({ group, value }: { group: string; value: string }) {
  const { t } = useTranslation();
  return <Tag>{t(`portal.school_lms.status.${group}.${value}`)}</Tag>;
}

/** antd rules + input props (hard cap and counter) for a field limited by the OpenAPI spec. */
export function useFieldRules() {
  const { t } = useTranslation(OPS_UI_NS);
  return {
    rules: (schema: OpenapiStringSchema, required = false) => openapiRules(schema, { required, t }),
    input: (schema: OpenapiStringSchema) => openapiInputProps(schema),
  };
}
