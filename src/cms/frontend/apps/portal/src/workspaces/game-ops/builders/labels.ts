// Field labels of the generated forms (AGENTS §7: UI strings from the translation files): `gameOps.field.<key>` when
// translated, otherwise the property name made readable (`max_level` → "max level") — schema property names are the
// technical vocabulary of the game designers (GLOSSARY).
import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';

export function useFieldLabel() {
  const { t, i18n } = useTranslation();
  return useCallback((key: string) => (i18n.exists(`gameOps.field.${key}`) ? t(`gameOps.field.${key}`) : key.replace(/_/g, ' ')), [t, i18n]);
}
