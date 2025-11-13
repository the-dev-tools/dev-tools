import { eq, useLiveQuery } from '@tanstack/react-db';
import { Ulid } from 'id128';
import { useDragAndDrop } from 'react-aria-components';
import { HttpHeader } from '@the-dev-tools/spec/api/http/v1/http_pb';
import { HttpHeaderCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { DataTable } from '@the-dev-tools/ui/data-table';
import { DropIndicatorHorizontal } from '@the-dev-tools/ui/reorder';
import { Protobuf, useApiCollection } from '~/api-new';
import {
  columnActionsCommon,
  columnCheckboxField,
  columnReferenceField,
  columnTextField,
  ReactTableNoMemo,
  useFormTable,
  useFormTableAddRow,
} from '~/form-table';
import { getNextOrder, handleCollectionReorder } from '~/utils/order';

export interface HeaderTableProps {
  httpId: Uint8Array;
}

export const HeaderTable = ({ httpId }: HeaderTableProps) => {
  const collection = useApiCollection(HttpHeaderCollectionSchema);

  const { data: items } = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where((_) => eq(_.item.httpId, httpId))
        .orderBy((_) => _.item.order),
    [collection, httpId],
  );

  const formTable = useFormTable<HttpHeader>({
    onUpdate: (_) => collection.utils.update(Protobuf.messageData(_)),
  });

  const addRow = useFormTableAddRow({
    createLabel: 'New header',
    items,
    onCreate: async () =>
      void collection.utils.insert({
        enabled: true,
        httpHeaderId: Ulid.generate().bytes,
        httpId,
        order: await getNextOrder(collection),
      }),
    primaryColumn: 'key',
  });

  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) => [...keys].map((key) => ({ key: key.toString() })),
    onReorder: handleCollectionReorder(collection),
    renderDropIndicator: () => <DropIndicatorHorizontal as='tr' />,
  });

  return (
    <ReactTableNoMemo
      columns={[
        columnCheckboxField<HttpHeader>('enabled', { meta: { divider: false } }),
        columnReferenceField<HttpHeader>('key', { meta: { isRowHeader: true } }),
        columnReferenceField<HttpHeader>('value', { allowFiles: true }),
        columnTextField<HttpHeader>('description', { meta: { divider: false } }),
        columnActionsCommon<HttpHeader>({
          onDelete: (_) => collection.utils.delete(Protobuf.messageData(_)),
        }),
      ]}
      data={items}
      getRowId={(_) => collection.utils.getKey(_)}
    >
      {(table) => (
        <DataTable {...formTable} {...addRow} aria-label='Headers' dragAndDropHooks={dragAndDropHooks} table={table} />
      )}
    </ReactTableNoMemo>
  );
};
