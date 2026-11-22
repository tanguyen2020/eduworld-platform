// SearchSelect — the only dropdown of the Admin Portal (owner rule 2026-11-22, ADMIN_PORTAL §1.6, ADR-0047). A plain
// antd Select with: `showSearch`, client-side filtering that ignores case and Vietnamese diacritics (viFilterOption),
// a clear empty state, and a visible warning when the option list was cut by the safety cap of `fetchAllPages`
// (`truncated`). Options are the WHOLE list (portal `fetchAllPages`); never feed it a single API page. Props given by the
// caller win (e.g. `filterOption={false}` + `onSearch` for a server-side search).
import { Alert, Empty, Select, type SelectProps } from 'antd';
import type { RefSelectProps } from 'antd/es/select';
import { forwardRef, type ReactElement, type Ref } from 'react';
import { useTranslation } from 'react-i18next';
import { OPS_UI_NS } from '../i18n';
import { viFilterOption } from './viSearch';

export type SearchSelectProps<V = unknown> = SelectProps<V> & {
  /** The option list was cut by the safety cap of `fetchAllPages`: warn instead of pretending it is complete. */
  truncated?: boolean;
};

function SearchSelectInner<V = unknown>({ truncated, popupRender, dropdownRender, ...props }: SearchSelectProps<V>, ref: Ref<RefSelectProps>) {
  const { t } = useTranslation(OPS_UI_NS);
  const render = popupRender ?? dropdownRender;
  const withNotice = (menu: ReactElement): ReactElement => {
    const inner = render ? (render(menu) as ReactElement) : menu;
    if (!truncated) return inner;
    return (
      <>
        {inner}
        <Alert type="warning" showIcon banner role="status" data-testid="select-truncated" message={t('select.truncated')} />
      </>
    );
  };
  return (
    <Select<V>
      ref={ref}
      showSearch
      filterOption={viFilterOption}
      notFoundContent={<Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('select.empty')} />}
      status={truncated ? 'warning' : undefined}
      {...props}
      {...(render || truncated ? { popupRender: withNotice } : {})}
    />
  );
}

export const SearchSelect = forwardRef(SearchSelectInner) as <V = unknown>(
  props: SearchSelectProps<V> & { ref?: Ref<RefSelectProps> },
) => ReactElement;
