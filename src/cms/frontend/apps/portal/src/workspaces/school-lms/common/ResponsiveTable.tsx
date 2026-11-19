// ResponsiveTable (F052-AC-27, ADMIN_PORTAL §1.5): antd Table from 768 px, a list of cards below (each card shows the
// columns as label: value). Cursor pagination is driven by the caller ("Xem thêm"). Layout only, no data logic.
// [Proposed] move to @eduworld/ops-ui once a second workspace needs it (plan F052 lists it as an ops-ui component).
import { Button, Card, Descriptions, Flex, List, Table, theme } from 'antd';
import type { TableProps } from 'antd';
import type { ColumnType } from 'antd/es/table';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { useMediaQuery } from '../../../shell/useMediaQuery';

export const MOBILE_QUERY = '(min-width: 768px)';

export interface ResponsiveTableProps<T> {
  rowKey: keyof T & string;
  columns: ColumnType<T>[];
  data: readonly T[];
  loading?: boolean;
  /** Title of a card (mobile), usually the name column. */
  cardTitle: (row: T) => ReactNode;
  hasMore?: boolean;
  onLoadMore?: () => void;
  rowSelection?: TableProps<T>['rowSelection'];
  caption?: string;
}

export function ResponsiveTable<T extends object>({ rowKey, columns, data, loading, cardTitle, hasMore, onLoadMore, rowSelection, caption }: ResponsiveTableProps<T>) {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const desktop = useMediaQuery(MOBILE_QUERY);
  const more = hasMore && onLoadMore ? (
    <Flex justify="center" style={{ marginTop: token.marginSM }}>
      <Button onClick={onLoadMore} loading={loading}>
        {t('portal.school_lms.common.loadMore')}
      </Button>
    </Flex>
  ) : null;
  if (desktop) {
    return (
      <>
        <Table<T>
          rowKey={rowKey}
          columns={columns}
          dataSource={data as T[]}
          loading={loading}
          pagination={false}
          rowSelection={rowSelection}
          scroll={{ x: true }}
          size="middle"
          {...(caption ? { title: () => caption } : {})}
        />
        {more}
      </>
    );
  }
  return (
    <>
      <List<T>
        data-testid="responsive-cards"
        loading={loading}
        dataSource={data as T[]}
        renderItem={(row) => (
          <List.Item key={String(row[rowKey])}>
            <Card size="small" title={cardTitle(row)} style={{ width: '100%' }}>
              <Descriptions size="small" column={1}>
                {columns
                  .filter((c) => c.key !== 'title')
                  .map((c, i) => (
                    <Descriptions.Item key={String(c.key ?? i)} label={c.title as ReactNode}>
                      {c.render ? (c.render((c.dataIndex ? row[c.dataIndex as keyof T] : undefined) as never, row, i) as ReactNode) : String(row[c.dataIndex as keyof T] ?? '')}
                    </Descriptions.Item>
                  ))}
              </Descriptions>
            </Card>
          </List.Item>
        )}
      />
      {more}
    </>
  );
}
