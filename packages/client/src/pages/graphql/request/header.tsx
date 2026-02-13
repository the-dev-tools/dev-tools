import { eq, useLiveQuery } from '@tanstack/react-db';
import { Ulid } from 'id128';
import { useDragAndDrop } from 'react-aria-components';
import { GraphQLHeaderCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/graph_q_l';
import { DataTable } from '@the-dev-tools/ui/data-table';
import { DropIndicatorHorizontal } from '@the-dev-tools/ui/reorder';
import {
  columnActionsCommon,
  columnCheckboxField,
  columnReferenceField,
  columnTextField,
  ReactTableNoMemo,
  useFormTableAddRow,
} from '~/features/form-table';
import { useApiCollection } from '~/shared/api';
import { getNextOrder, handleCollectionReorder, pick } from '~/shared/lib';

export interface GraphQLHeaderTableProps {
  graphqlId: Uint8Array;
}

export const GraphQLHeaderTable = ({ graphqlId }: GraphQLHeaderTableProps) => {
  const collection = useApiCollection(GraphQLHeaderCollectionSchema);

  const items = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where((_) => eq(_.item.graphqlId, graphqlId))
        .orderBy((_) => _.item.order)
        .select((_) => pick(_.item, 'graphqlHeaderId', 'order')),
    [collection, graphqlId],
  ).data.map((_) => pick(_, 'graphqlHeaderId'));

  const addRow = useFormTableAddRow({
    createLabel: 'New header',
    items,
    onCreate: async () =>
      void collection.utils.insert({
        enabled: true,
        graphqlHeaderId: Ulid.generate().bytes,
        graphqlId,
        order: await getNextOrder(collection),
      }),
  });

  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) => [...keys].map((key) => ({ key: key.toString() })),
    onReorder: handleCollectionReorder(collection),
    renderDropIndicator: () => <DropIndicatorHorizontal as='tr' />,
  });

  const getItem = (row: (typeof items)[number]) =>
    collection.get(collection.utils.getKey({ graphqlHeaderId: row.graphqlHeaderId }));

  return (
    <ReactTableNoMemo
      columns={[
        columnCheckboxField('enabled', {
          onChange: (value, context) => {
            const item = getItem(context.row.original);
            if (item) collection.utils.update({ enabled: value, graphqlHeaderId: item.graphqlHeaderId });
          },
          value: (provide, context) => {
            const item = getItem(context.row.original);
            return provide(item?.enabled ?? false);
          },
        }),
        columnReferenceField(
          'key',
          {
            onChange: (value, context) => {
              const item = getItem(context.row.original);
              if (item) collection.utils.update({ graphqlHeaderId: item.graphqlHeaderId, key: value });
            },
            value: (provide, context) => {
              const item = getItem(context.row.original);
              return provide(item?.key ?? '');
            },
          },
          { meta: { isRowHeader: true } },
        ),
        columnReferenceField('value', {
          onChange: (value, context) => {
            const item = getItem(context.row.original);
            if (item) collection.utils.update({ graphqlHeaderId: item.graphqlHeaderId, value });
          },
          value: (provide, context) => {
            const item = getItem(context.row.original);
            return provide(item?.value ?? '');
          },
        }),
        columnTextField('description', {
          onChange: (value, context) => {
            const item = getItem(context.row.original);
            if (item) collection.utils.update({ description: value, graphqlHeaderId: item.graphqlHeaderId });
          },
          value: (provide, context) => {
            const item = getItem(context.row.original);
            return provide(item?.description ?? '');
          },
        }),
        columnActionsCommon({
          onDelete: (item) => collection.utils.delete({ graphqlHeaderId: item.graphqlHeaderId! }),
        }),
      ]}
      data={items}
      getRowId={(_) => collection.utils.getKey({ graphqlHeaderId: _.graphqlHeaderId! })}
    >
      {(table) => (
        <DataTable {...addRow} aria-label='Headers' dragAndDropHooks={dragAndDropHooks} table={table} />
      )}
    </ReactTableNoMemo>
  );
};
