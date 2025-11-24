import { eq, or, useLiveQuery } from '@tanstack/react-db';
import { Ulid } from 'id128';
import { useDragAndDrop } from 'react-aria-components';
import {
  HttpSearchParamCollectionSchema,
  HttpSearchParamDeltaCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { DataTable } from '@the-dev-tools/ui/data-table';
import { DropIndicatorHorizontal } from '@the-dev-tools/ui/reorder';
import { useApiCollection } from '~/api-new';
import { ReactTableNoMemo, useFormTableAddRow } from '~/form-table';
import { deltaActionsColumn, deltaCheckboxColumn, deltaReferenceColumn, deltaTextFieldColumn } from '~/utils/delta';
import { getNextOrder, handleCollectionReorder } from '~/utils/order';
import { pick } from '~/utils/tanstack-db';

export interface SearchParamTableProps {
  deltaHttpId: Uint8Array | undefined;
  httpId: Uint8Array;
  isReadOnly?: boolean;
}

export const SearchParamTable = ({ deltaHttpId, httpId, isReadOnly = false }: SearchParamTableProps) => {
  const collection = useApiCollection(HttpSearchParamCollectionSchema);

  const items = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where((_) => or(eq(_.item.httpId, httpId), eq(_.item.httpId, deltaHttpId)))
        .orderBy((_) => _.item.order)
        .select((_) => pick(_.item, 'httpSearchParamId', 'order')),
    [collection, deltaHttpId, httpId],
  ).data.map((_) => pick(_, 'httpSearchParamId'));

  const deltaColumnOptions = {
    deltaKey: 'deltaHttpSearchParamId',
    deltaParentKey: { httpId: deltaHttpId },
    deltaSchema: HttpSearchParamDeltaCollectionSchema,
    isDelta: deltaHttpId !== undefined,
    originKey: 'httpSearchParamId',
    originSchema: HttpSearchParamCollectionSchema,
  } as const;

  const addRow = useFormTableAddRow({
    createLabel: 'New search param',
    items,
    onCreate: async () =>
      void collection.utils.insert({
        enabled: true,
        httpId: deltaHttpId ?? httpId,
        httpSearchParamId: Ulid.generate().bytes,
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
      getRowId={(_) => collection.utils.getKey({ httpSearchParamId: _.httpSearchParamId! })}
    >
      {(table) => (
        <DataTable
          {...(!isReadOnly && addRow)}
          aria-label='Search params'
          dragAndDropHooks={dragAndDropHooks}
          table={table}
        />
      )}
    </ReactTableNoMemo>
  );
};
