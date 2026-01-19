import { eq, or, useLiveQuery } from '@tanstack/react-db';
import { Ulid } from 'id128';
import { useDragAndDrop } from 'react-aria-components';
import {
  HttpBodyUrlEncodedCollectionSchema,
  HttpBodyUrlEncodedDeltaCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { DataTable } from '@the-dev-tools/ui/data-table';
import { DropIndicatorHorizontal } from '@the-dev-tools/ui/reorder';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { deltaActionsColumn, deltaCheckboxColumn, deltaReferenceColumn, deltaTextFieldColumn } from '~/features/delta';
import { ReactTableNoMemo, useFormTableAddRow } from '~/features/form-table';
import { useApiCollection } from '~/shared/api';
import { getNextOrder, handleCollectionReorder, pick } from '~/shared/lib';

export interface BodyUrlEncodedTableProps {
  deltaHttpId: Uint8Array | undefined;
  hideDescription?: boolean;
  httpId: Uint8Array;
  isReadOnly?: boolean;
}

export const BodyUrlEncodedTable = ({
  deltaHttpId,
  hideDescription = false,
  httpId,
  isReadOnly = false,
}: BodyUrlEncodedTableProps) => {
  const collection = useApiCollection(HttpBodyUrlEncodedCollectionSchema);

  const items = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where((_) => or(eq(_.item.httpId, httpId), eq(_.item.httpId, deltaHttpId)))
        .orderBy((_) => _.item.order)
        .select((_) => pick(_.item, 'httpBodyUrlEncodedId', 'order')),
    [collection, deltaHttpId, httpId],
  ).data.map((_) => pick(_, 'httpBodyUrlEncodedId'));

  const deltaColumnOptions = {
    deltaKey: 'deltaHttpBodyUrlEncodedId',
    deltaParentKey: { httpId: deltaHttpId },
    deltaSchema: HttpBodyUrlEncodedDeltaCollectionSchema,
    isDelta: deltaHttpId !== undefined,
    originKey: 'httpBodyUrlEncodedId',
    originSchema: HttpBodyUrlEncodedCollectionSchema,
  } as const;

  const addRow = useFormTableAddRow({
    createLabel: 'New body item',
    items,
    onCreate: async () =>
      void collection.utils.insert({
        enabled: true,
        httpBodyUrlEncodedId: Ulid.generate().bytes,
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
        ...(hideDescription
          ? []
          : [deltaTextFieldColumn({ ...deltaColumnOptions, isReadOnly, valueKey: 'description' })]),
        ...(isReadOnly ? [] : [deltaActionsColumn(deltaColumnOptions)]),
      ]}
      data={items}
      getRowId={(_) => collection.utils.getKey({ httpBodyUrlEncodedId: _.httpBodyUrlEncodedId! })}
    >
      {(table) => (
        <DataTable
          {...(!isReadOnly && addRow)}
          aria-label='Body items'
          containerClassName={tw`col-span-full`}
          dragAndDropHooks={dragAndDropHooks}
          table={table}
        />
      )}
    </ReactTableNoMemo>
  );
};
