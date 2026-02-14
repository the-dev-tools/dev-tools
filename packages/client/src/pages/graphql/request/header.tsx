import { eq, useLiveQuery } from '@tanstack/react-db';
import { Ulid } from 'id128';
import { useDragAndDrop } from 'react-aria-components';
import { FiPlus } from 'react-icons/fi';
import { GraphQLHeaderCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/graph_q_l';
import { Button } from '@the-dev-tools/ui/button';
import { Checkbox } from '@the-dev-tools/ui/checkbox';
import { DropIndicatorHorizontal } from '@the-dev-tools/ui/reorder';
import { Table, TableBody, TableCell, TableColumn, TableFooter, TableHeader, TableRow } from '@the-dev-tools/ui/table';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField } from '@the-dev-tools/ui/text-field';
import { ReferenceField } from '~/features/expression';
import { ColumnActionDelete } from '~/features/form-table';
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

  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) => [...keys].map((key) => ({ key: key.toString() })),
    onReorder: handleCollectionReorder(collection),
    renderDropIndicator: () => <DropIndicatorHorizontal as='tr' />,
  });

  return (
    <Table aria-label='Headers' dragAndDropHooks={dragAndDropHooks}>
      <TableHeader>
        <TableColumn width={32} />
        <TableColumn isRowHeader>Key</TableColumn>
        <TableColumn>Value</TableColumn>
        <TableColumn>Description</TableColumn>
        <TableColumn width={32} />
      </TableHeader>

      <TableBody items={items}>
        {({ graphqlHeaderId }) => {
          const item = collection.get(collection.utils.getKey({ graphqlHeaderId }));

          return (
            <TableRow id={collection.utils.getKey({ graphqlHeaderId })}>
              <TableCell className={tw`border-r-0`}>
                <div className={tw`flex flex-1 px-1`}>
                  <Checkbox
                    isSelected={item?.enabled ?? false}
                    isTableCell
                    onChange={(enabled) => void collection.utils.update({ enabled, graphqlHeaderId })}
                  />
                </div>
              </TableCell>

              <TableCell>
                <ReferenceField
                  allowFiles
                  className='flex-1'
                  kind='StringExpression'
                  onChange={(key) => void collection.utils.updatePaced({ graphqlHeaderId, key })}
                  placeholder='Enter key'
                  value={item?.key ?? ''}
                  variant='table-cell'
                />
              </TableCell>

              <TableCell>
                <ReferenceField
                  allowFiles
                  className='flex-1'
                  kind='StringExpression'
                  onChange={(value) => void collection.utils.updatePaced({ graphqlHeaderId, value })}
                  placeholder='Enter value'
                  value={item?.value ?? ''}
                  variant='table-cell'
                />
              </TableCell>

              <TableCell>
                <TextInputField
                  aria-label='description'
                  className={tw`flex-1`}
                  isTableCell
                  onChange={(description) => void collection.utils.updatePaced({ description, graphqlHeaderId })}
                  placeholder='Enter description'
                  value={item?.description ?? ''}
                />
              </TableCell>

              <TableCell className={tw`border-r-0 px-1`}>
                <ColumnActionDelete onDelete={() => void collection.utils.delete({ graphqlHeaderId })} />
              </TableCell>
            </TableRow>
          );
        }}
      </TableBody>

      <TableFooter>
        <Button
          className={tw`w-full justify-start -outline-offset-4`}
          onPress={async () => {
            collection.utils.insert({
              enabled: true,
              graphqlHeaderId: Ulid.generate().bytes,
              graphqlId,
              order: await getNextOrder(collection),
            });
          }}
          variant='ghost'
        >
          <FiPlus className={tw`size-4 text-on-neutral-low`} />
          New header
        </Button>
      </TableFooter>
    </Table>
  );
};

