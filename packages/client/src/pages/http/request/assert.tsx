import { eq, or, useLiveQuery } from '@tanstack/react-db';
import { Ulid } from 'id128';
import { useDragAndDrop } from 'react-aria-components';
import {
  HttpAssertCollectionSchema,
  HttpAssertDeltaCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { DataTable } from '@the-dev-tools/ui/data-table';
import { DropIndicatorHorizontal } from '@the-dev-tools/ui/reorder';
import { deltaActionsColumn, deltaCheckboxColumn, deltaReferenceColumn } from '~/features/delta';
import { ReactTableNoMemo, useFormTableAddRow } from '~/features/form-table';
import { useApiCollection } from '~/shared/api';
import { getNextOrder, handleCollectionReorder, pick } from '~/shared/lib';

export interface AssertTableProps {
  deltaHttpId: Uint8Array | undefined;
  httpId: Uint8Array;
  isReadOnly?: boolean;
}

export const AssertTable = ({ deltaHttpId, httpId, isReadOnly = false }: AssertTableProps) => {
  const collection = useApiCollection(HttpAssertCollectionSchema);

  const items = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where((_) => or(eq(_.item.httpId, httpId), eq(_.item.httpId, deltaHttpId)))
        .orderBy((_) => _.item.order)
        .select((_) => pick(_.item, 'httpAssertId', 'order')),
    [collection, deltaHttpId, httpId],
  ).data.map((_) => pick(_, 'httpAssertId'));

  const deltaColumnOptions = {
    deltaKey: 'deltaHttpAssertId',
    deltaParentKey: { httpId: deltaHttpId },
    deltaSchema: HttpAssertDeltaCollectionSchema,
    isDelta: deltaHttpId !== undefined,
    originKey: 'httpAssertId',
    originSchema: HttpAssertCollectionSchema,
  } as const;

  const addRow = useFormTableAddRow({
    createLabel: 'New assertion',
    items,
    onCreate: async () =>
      void collection.utils.insert({
        enabled: true,
        httpAssertId: Ulid.generate().bytes,
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
        deltaReferenceColumn({
          ...deltaColumnOptions,
          allowFiles: true,
          fullExpression: true,
          isReadOnly,
          meta: { isRowHeader: true },
          valueKey: 'value',
        }),
        ...(isReadOnly ? [] : [deltaActionsColumn(deltaColumnOptions)]),
      ]}
      data={items}
      getRowId={(_) => collection.utils.getKey({ httpAssertId: _.httpAssertId! })}
    >
      {(table) => (
        <DataTable
          {...(!isReadOnly && addRow)}
          aria-label='Assertions'
          dragAndDropHooks={dragAndDropHooks}
          table={table}
        />
      )}
    </ReactTableNoMemo>
  );
};
