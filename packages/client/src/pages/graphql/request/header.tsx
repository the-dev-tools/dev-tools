import { eq, or, useLiveQuery } from '@tanstack/react-db';
import { Ulid } from 'id128';
import { useDragAndDrop } from 'react-aria-components';
import { FiPlus } from 'react-icons/fi';
import {
  GraphQLHeaderCollectionSchema,
  GraphQLHeaderDeltaCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/graph_q_l';
import { Button } from '@the-dev-tools/ui/button';
import { DropIndicatorHorizontal } from '@the-dev-tools/ui/reorder';
import { Table, TableBody, TableCell, TableColumn, TableFooter, TableHeader, TableRow } from '@the-dev-tools/ui/table';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { ColumnActionDeleteDelta, DeltaCheckbox, DeltaReference, DeltaTextField } from '~/features/delta';
import { useApiCollection } from '~/shared/api';
import { getNextOrder, handleCollectionReorder, pick } from '~/shared/lib';

export interface GraphQLHeaderTableProps {
  deltaGraphqlId?: Uint8Array | undefined;
  graphqlId: Uint8Array;
  hideDescription?: boolean;
  isReadOnly?: boolean;
}

export const GraphQLHeaderTable = ({
  deltaGraphqlId,
  graphqlId,
  hideDescription = false,
  isReadOnly = false,
}: GraphQLHeaderTableProps) => {
  const collection = useApiCollection(GraphQLHeaderCollectionSchema);

  const items = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where((_) => or(eq(_.item.graphqlId, graphqlId), eq(_.item.graphqlId, deltaGraphqlId)))
        .orderBy((_) => _.item.order)
        .select((_) => pick(_.item, 'graphqlHeaderId', 'order')),
    [collection, deltaGraphqlId, graphqlId],
  ).data.map((_) => pick(_, 'graphqlHeaderId'));

  const deltaColumnOptions = {
    deltaKey: 'deltaGraphqlHeaderId',
    deltaParentKey: { graphqlId: deltaGraphqlId },
    deltaSchema: GraphQLHeaderDeltaCollectionSchema,
    isDelta: deltaGraphqlId !== undefined,
    originKey: 'graphqlHeaderId',
    originSchema: GraphQLHeaderCollectionSchema,
  } as const;

  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) => [...keys].map((key) => ({ key: key.toString() })),
    onReorder: handleCollectionReorder(collection),
    renderDropIndicator: () => <DropIndicatorHorizontal as='tr' />,
  });

  return (
    <Table {...(!isReadOnly && { dragAndDropHooks })} aria-label='Headers'>
      <TableHeader>
        <TableColumn width={32} />
        <TableColumn isRowHeader>Key</TableColumn>
        <TableColumn>Value</TableColumn>
        {!hideDescription && <TableColumn>Description</TableColumn>}
        {!isReadOnly && <TableColumn width={32} />}
      </TableHeader>

      <TableBody items={items}>
        {({ graphqlHeaderId }) => (
          <TableRow id={collection.utils.getKey({ graphqlHeaderId })}>
            <TableCell className={tw`border-r-0`}>
              <DeltaCheckbox
                {...deltaColumnOptions}
                isReadOnly={isReadOnly}
                originKeyObject={{ graphqlHeaderId }}
                valueKey='enabled'
              />
            </TableCell>

            <TableCell>
              <DeltaReference
                {...deltaColumnOptions}
                allowFiles
                isReadOnly={isReadOnly}
                originKeyObject={{ graphqlHeaderId }}
                valueKey='key'
              />
            </TableCell>

            <TableCell>
              <DeltaReference
                {...deltaColumnOptions}
                allowFiles
                isReadOnly={isReadOnly}
                originKeyObject={{ graphqlHeaderId }}
                valueKey='value'
              />
            </TableCell>

            {!hideDescription && (
              <TableCell>
                <DeltaTextField
                  {...deltaColumnOptions}
                  isReadOnly={isReadOnly}
                  originKeyObject={{ graphqlHeaderId }}
                  valueKey='description'
                />
              </TableCell>
            )}

            {!isReadOnly && (
              <TableCell className={tw`border-r-0 px-1`}>
                <ColumnActionDeleteDelta {...deltaColumnOptions} originKeyObject={{ graphqlHeaderId }} />
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
                graphqlHeaderId: Ulid.generate().bytes,
                graphqlId: deltaGraphqlId ?? graphqlId,
                order: await getNextOrder(collection),
              });
            }}
            variant='ghost'
          >
            <FiPlus className={tw`size-4 text-on-neutral-low`} />
            New header
          </Button>
        </TableFooter>
      )}
    </Table>
  );
};

