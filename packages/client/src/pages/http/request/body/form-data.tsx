import { eq, or, useLiveQuery } from '@tanstack/react-db';
import { Ulid } from 'id128';
import { useDragAndDrop } from 'react-aria-components';
import { FiPlus } from 'react-icons/fi';
import {
  HttpBodyFormDataCollectionSchema,
  HttpBodyFormDataDeltaCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { Button } from '@the-dev-tools/ui/button';
import { DropIndicatorHorizontal } from '@the-dev-tools/ui/reorder';
import { Table, TableBody, TableCell, TableColumn, TableFooter, TableHeader, TableRow } from '@the-dev-tools/ui/table';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { ColumnActionDeleteDelta, DeltaCheckbox, DeltaReference, DeltaTextField } from '~/features/delta';
import { useApiCollection } from '~/shared/api';
import { getNextOrder, handleCollectionReorder, pick } from '~/shared/lib';

export interface BodyFormDataTableProps {
  deltaHttpId: Uint8Array | undefined;
  hideDescription?: boolean;
  httpId: Uint8Array;
  isReadOnly?: boolean;
}

export const BodyFormDataTable = ({
  deltaHttpId,
  hideDescription = false,
  httpId,
  isReadOnly = false,
}: BodyFormDataTableProps) => {
  const collection = useApiCollection(HttpBodyFormDataCollectionSchema);

  const items = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where((_) => or(eq(_.item.httpId, httpId), eq(_.item.httpId, deltaHttpId)))
        .orderBy((_) => _.item.order)
        .select((_) => pick(_.item, 'httpBodyFormDataId', 'order')),
    [collection, deltaHttpId, httpId],
  ).data.map((_) => pick(_, 'httpBodyFormDataId'));

  const deltaColumnOptions = {
    deltaKey: 'deltaHttpBodyFormDataId',
    deltaParentKey: { httpId: deltaHttpId },
    deltaSchema: HttpBodyFormDataDeltaCollectionSchema,
    isDelta: deltaHttpId !== undefined,
    originKey: 'httpBodyFormDataId',
    originSchema: HttpBodyFormDataCollectionSchema,
  } as const;

  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) => [...keys].map((key) => ({ key: key.toString() })),
    onReorder: handleCollectionReorder(collection),
    renderDropIndicator: () => <DropIndicatorHorizontal as='tr' />,
  });

  return (
    <Table {...(!isReadOnly && { dragAndDropHooks })} aria-label='Body items' containerClassName={tw`col-span-full`}>
      <TableHeader>
        <TableColumn width={32} />
        <TableColumn isRowHeader>Key</TableColumn>
        <TableColumn>Value</TableColumn>
        {!hideDescription && <TableColumn>Description</TableColumn>}
        {!isReadOnly && <TableColumn width={32} />}
      </TableHeader>

      <TableBody items={items}>
        {({ httpBodyFormDataId }) => (
          <TableRow id={collection.utils.getKey({ httpBodyFormDataId })}>
            <TableCell className={tw`border-r-0`}>
              <DeltaCheckbox
                {...deltaColumnOptions}
                isReadOnly={isReadOnly}
                originKeyObject={{ httpBodyFormDataId }}
                valueKey='enabled'
              />
            </TableCell>

            <TableCell>
              <DeltaReference
                {...deltaColumnOptions}
                allowFiles
                isReadOnly={isReadOnly}
                originKeyObject={{ httpBodyFormDataId }}
                valueKey='key'
              />
            </TableCell>

            <TableCell>
              <DeltaReference
                {...deltaColumnOptions}
                allowFiles
                isReadOnly={isReadOnly}
                originKeyObject={{ httpBodyFormDataId }}
                valueKey='value'
              />
            </TableCell>

            {!hideDescription && (
              <TableCell>
                <DeltaTextField
                  {...deltaColumnOptions}
                  isReadOnly={isReadOnly}
                  originKeyObject={{ httpBodyFormDataId }}
                  valueKey='description'
                />
              </TableCell>
            )}

            {!isReadOnly && (
              <TableCell className={tw`border-r-0 px-1`}>
                <ColumnActionDeleteDelta {...deltaColumnOptions} originKeyObject={{ httpBodyFormDataId }} />
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
                httpBodyFormDataId: Ulid.generate().bytes,
                httpId: deltaHttpId ?? httpId,
                order: await getNextOrder(collection),
              });
            }}
            variant='ghost'
          >
            <FiPlus className={tw`size-4 text-on-neutral-low`} />
            New body item
          </Button>
        </TableFooter>
      )}
    </Table>
  );
};
