import { eq, or, useLiveQuery } from '@tanstack/react-db';
import { Ulid } from 'id128';
import { useDragAndDrop } from 'react-aria-components';
import { FiPlus } from 'react-icons/fi';
import {
  GraphQLAssertCollectionSchema,
  GraphQLAssertDeltaCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/graph_q_l';
import { Button } from '@the-dev-tools/ui/button';
import { DropIndicatorHorizontal } from '@the-dev-tools/ui/reorder';
import { Table, TableBody, TableCell, TableColumn, TableFooter, TableHeader, TableRow } from '@the-dev-tools/ui/table';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { ColumnActionDeleteDelta, DeltaCheckbox, DeltaReference } from '~/features/delta';
import { useApiCollection } from '~/shared/api';
import { getNextOrder, handleCollectionReorder, pick } from '~/shared/lib';

export interface GraphQLAssertTableProps {
  deltaGraphqlId?: Uint8Array | undefined;
  graphqlId: Uint8Array;
  isReadOnly?: boolean;
}

export const GraphQLAssertTable = ({
  deltaGraphqlId,
  graphqlId,
  isReadOnly = false,
}: GraphQLAssertTableProps) => {
  const collection = useApiCollection(GraphQLAssertCollectionSchema);

  const items = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where((_) => or(eq(_.item.graphqlId, graphqlId), eq(_.item.graphqlId, deltaGraphqlId)))
        .orderBy((_) => _.item.order)
        .select((_) => pick(_.item, 'graphqlAssertId', 'order')),
    [collection, deltaGraphqlId, graphqlId],
  ).data.map((_) => pick(_, 'graphqlAssertId'));

  const deltaColumnOptions = {
    deltaKey: 'deltaGraphqlAssertId',
    deltaParentKey: { graphqlId: deltaGraphqlId },
    deltaSchema: GraphQLAssertDeltaCollectionSchema,
    isDelta: deltaGraphqlId !== undefined,
    originKey: 'graphqlAssertId',
    originSchema: GraphQLAssertCollectionSchema,
  } as const;

  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) => [...keys].map((key) => ({ key: key.toString() })),
    onReorder: handleCollectionReorder(collection),
    renderDropIndicator: () => <DropIndicatorHorizontal as='tr' />,
  });

  return (
    <Table {...(!isReadOnly && { dragAndDropHooks })} aria-label='Assertions'>
      <TableHeader>
        <TableColumn width={32} />
        <TableColumn isRowHeader>Value</TableColumn>
        {!isReadOnly && <TableColumn width={32} />}
      </TableHeader>

      <TableBody items={items}>
        {({ graphqlAssertId }) => (
          <TableRow id={collection.utils.getKey({ graphqlAssertId })}>
            <TableCell className={tw`border-r-0`}>
              <DeltaCheckbox
                {...deltaColumnOptions}
                isReadOnly={isReadOnly}
                originKeyObject={{ graphqlAssertId }}
                valueKey='enabled'
              />
            </TableCell>

            <TableCell>
              <DeltaReference
                {...deltaColumnOptions}
                allowFiles
                fullExpression
                isReadOnly={isReadOnly}
                originKeyObject={{ graphqlAssertId }}
                valueKey='value'
              />
            </TableCell>

            {!isReadOnly && (
              <TableCell className={tw`border-r-0 px-1`}>
                <ColumnActionDeleteDelta {...deltaColumnOptions} originKeyObject={{ graphqlAssertId }} />
              </TableCell>
            )}
          </TableRow>
        )}
      </TableBody>

      {!isReadOnly && (
        <TableFooter>
          <Button
            className={tw`w-full justify-start -outline-offset-4`}
            onPress={async () => {
              collection.utils.insert({
                enabled: true,
                graphqlAssertId: Ulid.generate().bytes,
                graphqlId: deltaGraphqlId ?? graphqlId,
                order: await getNextOrder(collection),
              });
            }}
            variant='ghost'
          >
            <FiPlus className={tw`size-4 text-on-neutral-low`} />
            New assertion
          </Button>
        </TableFooter>
      )}
    </Table>
  );
};
