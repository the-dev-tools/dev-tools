import { eq, or, useLiveQuery } from '@tanstack/react-db';
import { Ulid } from 'id128';
import { useDragAndDrop } from 'react-aria-components';
import { FiPlus } from 'react-icons/fi';
import {
  HttpSearchParamCollectionSchema,
  HttpSearchParamDeltaCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { Button } from '@the-dev-tools/ui/button';
import { DropIndicatorHorizontal } from '@the-dev-tools/ui/reorder';
import { Table, TableBody, TableCell, TableColumn, TableFooter, TableHeader, TableRow } from '@the-dev-tools/ui/table';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { ColumnActionDeleteDelta, DeltaCheckbox, DeltaReference, DeltaTextField } from '~/features/delta';
import { useApiCollection } from '~/shared/api';
import { getNextOrder, handleCollectionReorder, pick } from '~/shared/lib';

export interface SearchParamTableProps {
  deltaHttpId: Uint8Array | undefined;
  hideDescription?: boolean;
  httpId: Uint8Array;
  isReadOnly?: boolean;
}

export const SearchParamTable = ({
  deltaHttpId,
  hideDescription = false,
  httpId,
  isReadOnly = false,
}: SearchParamTableProps) => {
  const collection = useApiCollection(HttpSearchParamCollectionSchema);

  const items = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where((_) => or(eq(_.item.httpId, httpId), eq(_.item.httpId, deltaHttpId)))
        .orderBy((_) => _.item.order)
        .select((_) => pick(_.item, 'httpSearchParamId', 'order')),
    [collection, deltaHttpId, httpId],
  ).data.map((_) => pick(_, 'httpSearchParamId'));

  const deltaColumnOptions = {
    deltaKey: 'deltaHttpSearchParamId',
    deltaParentKey: { httpId: deltaHttpId },
    deltaSchema: HttpSearchParamDeltaCollectionSchema,
    isDelta: deltaHttpId !== undefined,
    originKey: 'httpSearchParamId',
    originSchema: HttpSearchParamCollectionSchema,
  } as const;

  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) => [...keys].map((key) => ({ key: key.toString() })),
    onReorder: handleCollectionReorder(collection),
    renderDropIndicator: () => <DropIndicatorHorizontal as='tr' />,
  });

  return (
    <Table {...(!isReadOnly && { dragAndDropHooks })} aria-label='Search params'>
      <TableHeader>
        <TableColumn width={32} />
        <TableColumn isRowHeader>Key</TableColumn>
        <TableColumn>Value</TableColumn>
        {!hideDescription && <TableColumn>Description</TableColumn>}
        {!isReadOnly && <TableColumn width={32} />}
      </TableHeader>

      <TableBody items={items}>
        {({ httpSearchParamId }) => (
          <TableRow id={collection.utils.getKey({ httpSearchParamId })}>
            <TableCell className={tw`border-r-0`}>
              <DeltaCheckbox
                {...deltaColumnOptions}
                isReadOnly={isReadOnly}
                originKeyObject={{ httpSearchParamId }}
                valueKey='enabled'
              />
            </TableCell>

            <TableCell>
              <DeltaReference
                {...deltaColumnOptions}
                allowFiles
                isReadOnly={isReadOnly}
                originKeyObject={{ httpSearchParamId }}
                valueKey='key'
              />
            </TableCell>

            <TableCell>
              <DeltaReference
                {...deltaColumnOptions}
                allowFiles
                isReadOnly={isReadOnly}
                originKeyObject={{ httpSearchParamId }}
                valueKey='value'
              />
            </TableCell>

            {!hideDescription && (
              <TableCell>
                <DeltaTextField
                  {...deltaColumnOptions}
                  isReadOnly={isReadOnly}
                  originKeyObject={{ httpSearchParamId }}
                  valueKey='description'
                />
              </TableCell>
            )}

            {!isReadOnly && (
              <TableCell className={tw`border-r-0 px-1`}>
                <ColumnActionDeleteDelta {...deltaColumnOptions} originKeyObject={{ httpSearchParamId }} />
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
                httpId: deltaHttpId ?? httpId,
                httpSearchParamId: Ulid.generate().bytes,
                order: await getNextOrder(collection),
              });
            }}
            variant='ghost'
          >
            <FiPlus className={tw`size-4 text-slate-500`} />
            New search param
          </Button>
        </TableFooter>
      )}
    </Table>
  );
};
