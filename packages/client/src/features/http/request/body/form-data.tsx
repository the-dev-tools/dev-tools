import { eq, useLiveQuery } from '@tanstack/react-db';
import { Ulid } from 'id128';
import { useDragAndDrop } from 'react-aria-components';
import { HttpBodyFormData } from '@the-dev-tools/spec/api/http/v1/http_pb';
import { HttpBodyFormDataCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { DataTable, useReactTable } from '@the-dev-tools/ui/data-table';
import { DropIndicatorHorizontal } from '@the-dev-tools/ui/reorder';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { Protobuf, useApiCollection } from '~/api-new';
import {
  columnActionsCommon,
  columnCheckboxField,
  columnReferenceField,
  columnTextField,
  displayTable,
  ReactTableNoMemo,
  useFormTable,
  useFormTableAddRow,
} from '~/form-table';
import { getNextOrder, handleCollectionReorder } from '~/utils/order';

const dataColumns = [
  columnCheckboxField<HttpBodyFormData>('enabled', { meta: { divider: false } }),
  columnReferenceField<HttpBodyFormData>('key', { meta: { isRowHeader: true } }),
  columnReferenceField<HttpBodyFormData>('value', { allowFiles: true }),
  columnTextField<HttpBodyFormData>('description', { meta: { divider: false } }),
];

export interface FormDataTableProps {
  httpId: Uint8Array;
  isReadOnly?: boolean;
}

export const FormDataTable = ({ httpId, isReadOnly = false }: FormDataTableProps) => {
  if (isReadOnly) return <DisplayTable httpId={httpId} />;
  return <EditTable httpId={httpId} />;
};

interface DisplayTableProps {
  httpId: Uint8Array;
}

const DisplayTable = ({ httpId }: DisplayTableProps) => {
  const collection = useApiCollection(HttpBodyFormDataCollectionSchema);

  const { data: items } = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where((_) => eq(_.item.httpId, httpId))
        .orderBy((_) => _.item.order),
    [collection, httpId],
  );

  const table = useReactTable({
    columns: dataColumns,
    data: items,
  });

  return <DataTable {...displayTable<HttpBodyFormData>()} aria-label='Body items' table={table} />;
};

interface EditTableProps {
  httpId: Uint8Array;
}

const EditTable = ({ httpId }: EditTableProps) => {
  const collection = useApiCollection(HttpBodyFormDataCollectionSchema);

  const { data: items } = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where((_) => eq(_.item.httpId, httpId))
        .orderBy((_) => _.item.order),
    [collection, httpId],
  );

  const formTable = useFormTable<HttpBodyFormData>({
    onUpdate: (_) => collection.utils.update(Protobuf.messageData(_)),
  });

  const addRow = useFormTableAddRow({
    createLabel: 'New body item',
    items,
    onCreate: async () =>
      void collection.utils.insert({
        enabled: true,
        httpBodyFormDataId: Ulid.generate().bytes,
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
        ...dataColumns,
        columnActionsCommon<HttpBodyFormData>({
          onDelete: (_) => collection.utils.delete(Protobuf.messageData(_)),
        }),
      ]}
      data={items}
      getRowId={(_) => collection.utils.getKey(_)}
    >
      {(table) => (
        <DataTable
          {...formTable}
          {...addRow}
          aria-label='Body items'
          containerClassName={tw`col-span-full`}
          dragAndDropHooks={dragAndDropHooks}
          table={table}
        />
      )}
    </ReactTableNoMemo>
  );
};
