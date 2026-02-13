import { eq, or, useLiveQuery } from '@tanstack/react-db';
import { Ulid } from 'id128';
import { useDragAndDrop } from 'react-aria-components';
import { FiPlus } from 'react-icons/fi';
import {
  HttpHeaderCollectionSchema,
  HttpHeaderDeltaCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { Button } from '@the-dev-tools/ui/button';
import { DropIndicatorHorizontal } from '@the-dev-tools/ui/reorder';
import { Table, TableBody, TableCell, TableColumn, TableFooter, TableHeader, TableRow } from '@the-dev-tools/ui/table';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { ColumnActionDeleteDelta, DeltaCheckbox, DeltaReference, DeltaTextField } from '~/features/delta';
import { useApiCollection } from '~/shared/api';
import { getNextOrder, handleCollectionReorder, pick } from '~/shared/lib';

export interface HeaderTableProps {
  deltaHttpId: Uint8Array | undefined;
  hideDescription?: boolean;
  httpId: Uint8Array;
  isReadOnly?: boolean;
}

export const HeaderTable = ({ deltaHttpId, hideDescription = false, httpId, isReadOnly = false }: HeaderTableProps) => {
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
        {({ httpHeaderId }) => (
          <TableRow id={collection.utils.getKey({ httpHeaderId })}>
            <TableCell className={tw`border-r-0`}>
              <DeltaCheckbox
                {...deltaColumnOptions}
                isReadOnly={isReadOnly}
                originKeyObject={{ httpHeaderId }}
                valueKey='enabled'
              />
            </TableCell>

            <TableCell>
              <DeltaReference
                {...deltaColumnOptions}
                allowFiles
                isReadOnly={isReadOnly}
                originKeyObject={{ httpHeaderId }}
                valueKey='key'
              />
            </TableCell>

            <TableCell>
              <DeltaReference
                {...deltaColumnOptions}
                allowFiles
                isReadOnly={isReadOnly}
                originKeyObject={{ httpHeaderId }}
                valueKey='value'
              />
            </TableCell>

            {!hideDescription && (
              <TableCell>
                <DeltaTextField
                  {...deltaColumnOptions}
                  isReadOnly={isReadOnly}
                  originKeyObject={{ httpHeaderId }}
                  valueKey='description'
                />
              </TableCell>
            )}

            {!isReadOnly && (
              <TableCell className={tw`border-r-0 px-1`}>
                <ColumnActionDeleteDelta {...deltaColumnOptions} originKeyObject={{ httpHeaderId }} />
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
                httpHeaderId: Ulid.generate().bytes,
                httpId: deltaHttpId ?? httpId,
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
