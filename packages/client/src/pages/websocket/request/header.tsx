import { Query, useLiveQuery } from '@tanstack/react-db';
import { Ulid } from 'id128';
import { useDragAndDrop } from 'react-aria-components';
import { FiPlus } from 'react-icons/fi';
import { WebSocketHeaderCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/web_socket';
import { Button } from '@the-dev-tools/ui/button';
import { Checkbox } from '@the-dev-tools/ui/checkbox';
import { DropIndicatorHorizontal } from '@the-dev-tools/ui/reorder';
import { Table, TableBody, TableCell, TableColumn, TableFooter, TableHeader, TableRow } from '@the-dev-tools/ui/table';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField } from '@the-dev-tools/ui/text-field';
import { ReferenceField } from '~/features/expression';
import { ColumnActionDelete } from '~/features/form-table';
import { useApiCollection } from '~/shared/api';
import { eqStruct, getNextOrder, handleCollectionReorder, LiveQuery, pickStruct } from '~/shared/lib';

export interface WebSocketHeaderTableProps {
  websocketId: Uint8Array;
}

export const WebSocketHeaderTable = ({ websocketId }: WebSocketHeaderTableProps) => {
  const collection = useApiCollection(WebSocketHeaderCollectionSchema);

  const { data: items } = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where(eqStruct({ websocketId }))
        .orderBy((_) => _.item.order)
        .select(pickStruct('websocketHeaderId', 'order')),
    [collection, websocketId],
  );

  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) => [...keys].map((key) => ({ key: key.toString() })),
    onReorder: handleCollectionReorder(collection),
    renderDropIndicator: () => <DropIndicatorHorizontal as='tr' />,
  });

  return (
    <Table aria-label='WebSocket headers' dragAndDropHooks={dragAndDropHooks}>
      <TableHeader>
        <TableColumn width={32} />
        <TableColumn isRowHeader>Key</TableColumn>
        <TableColumn>Value</TableColumn>
        <TableColumn>Description</TableColumn>
        <TableColumn width={32} />
      </TableHeader>

      <TableBody items={items}>
        {({ websocketHeaderId }) => {
          const query = new Query().from({ item: collection }).where(eqStruct({ websocketHeaderId })).findOne();

          return (
            <TableRow id={collection.utils.getKey({ websocketHeaderId })}>
              <TableCell className={tw`border-r-0`}>
                <LiveQuery query={() => query.select(pickStruct('enabled'))}>
                  {({ data }) => (
                    <Checkbox
                      aria-label='Enabled'
                      isSelected={data?.enabled ?? false}
                      isTableCell
                      onChange={(_) => void collection.utils.update({ enabled: _, websocketHeaderId })}
                    />
                  )}
                </LiveQuery>
              </TableCell>

              <TableCell>
                <LiveQuery query={() => query.select(pickStruct('key'))}>
                  {({ data }) => (
                    <ReferenceField
                      className='flex-1'
                      kind='StringExpression'
                      onChange={(_) => void collection.utils.update({ key: _, websocketHeaderId })}
                      placeholder='Enter key'
                      value={data?.key ?? ''}
                      variant='table-cell'
                    />
                  )}
                </LiveQuery>
              </TableCell>

              <TableCell>
                <LiveQuery query={() => query.select(pickStruct('value'))}>
                  {({ data }) => (
                    <ReferenceField
                      className='flex-1'
                      kind='StringExpression'
                      onChange={(_) => void collection.utils.update({ value: _, websocketHeaderId })}
                      placeholder='Enter value'
                      value={data?.value ?? ''}
                      variant='table-cell'
                    />
                  )}
                </LiveQuery>
              </TableCell>

              <TableCell>
                <LiveQuery query={() => query.select(pickStruct('description'))}>
                  {({ data }) => (
                    <TextInputField
                      aria-label='Description'
                      className='flex-1'
                      isTableCell
                      onChange={(_) => void collection.utils.update({ description: _, websocketHeaderId })}
                      placeholder='Enter description'
                      value={data?.description ?? ''}
                    />
                  )}
                </LiveQuery>
              </TableCell>

              <TableCell className={tw`border-r-0 px-1`}>
                <ColumnActionDelete onDelete={() => void collection.utils.delete({ websocketHeaderId })} />
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
              order: await getNextOrder(collection),
              websocketHeaderId: Ulid.generate().bytes,
              websocketId,
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
