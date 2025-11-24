import { eq, or, useLiveQuery } from '@tanstack/react-db';
import { Ulid } from 'id128';
import { useDragAndDrop } from 'react-aria-components';
import {
  HttpHeaderCollectionSchema,
  HttpHeaderDeltaCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { DataTable } from '@the-dev-tools/ui/data-table';
import { DropIndicatorHorizontal } from '@the-dev-tools/ui/reorder';
import { useApiCollection } from '~/api-new';
import { ReactTableNoMemo, useFormTableAddRow } from '~/form-table';
import { deltaActionsColumn, deltaCheckboxColumn, deltaReferenceColumn, deltaTextFieldColumn } from '~/utils/delta';
import { getNextOrder, handleCollectionReorder } from '~/utils/order';
import { pick } from '~/utils/tanstack-db';

export interface HeaderTableProps {
  deltaHttpId: Uint8Array | undefined;
  httpId: Uint8Array;
  isReadOnly?: boolean;
}

export const HeaderTable = ({ deltaHttpId, httpId, isReadOnly = false }: HeaderTableProps) => {
  const collection = useApiCollection(HttpHeaderCollectionSchema);

  const items = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where((_) => or(eq(_.item.httpId, httpId), eq(_.item.httpId, deltaHttpId)))
        .orderBy((_) => _.item.order)
        .select((_) => pick(_.item, 'httpHeaderId', 'order')),
    [collection, deltaHttpId, httpId],
  ).data.map((_) => pick(_, 'httpHeaderId'));

  const deltaColumnOptions = {
    deltaKey: 'deltaHttpHeaderId',
    deltaParentKey: { httpId: deltaHttpId },
    deltaSchema: HttpHeaderDeltaCollectionSchema,
    isDelta: deltaHttpId !== undefined,
    originKey: 'httpHeaderId',
    originSchema: HttpHeaderCollectionSchema,
  } as const;

  const addRow = useFormTableAddRow({
    createLabel: 'New header',
    items,
    onCreate: async () =>
      void collection.utils.insert({
        enabled: true,
        httpHeaderId: Ulid.generate().bytes,
        httpId: deltaHttpId ?? httpId,
        order: await getNextOrder(collection),
      }),
  });

  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) => [...keys].map((key) => ({ key: key.toString() })),
    onReorder: handleCollectionReorder(collection),
    renderDropIndicator: () => <DropIndicatorHorizontal as='tr' />,
  });

  return (
    <ReactTableNoMemo
      columns={[
        deltaCheckboxColumn({ ...deltaColumnOptions, header: '', isReadOnly, valueKey: 'enabled' }),
        deltaReferenceColumn({ ...deltaColumnOptions, isReadOnly, meta: { isRowHeader: true }, valueKey: 'key' }),
        deltaReferenceColumn({ ...deltaColumnOptions, isReadOnly, valueKey: 'value' }),
        deltaTextFieldColumn({ ...deltaColumnOptions, isReadOnly, valueKey: 'description' }),
        ...(isReadOnly ? [] : [deltaActionsColumn(deltaColumnOptions)]),
      ]}
      data={items}
      getRowId={(_) => collection.utils.getKey({ httpHeaderId: _.httpHeaderId! })}
    >
      {(table) => (
        <DataTable
          {...(!isReadOnly && addRow)}
          aria-label='Headers'
          dragAndDropHooks={dragAndDropHooks}
          table={table}
        />
      )}
    </ReactTableNoMemo>
  );
};
