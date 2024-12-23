import { create, fromJson, toJson } from '@bufbuild/protobuf';
import {
  createConnectQueryKey,
  createProtobufSafeUpdater,
  useMutation as useConnectMutation,
  useSuspenseQuery as useConnectSuspenseQuery,
} from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import { createColumnHelper, getCoreRowModel, useReactTable } from '@tanstack/react-table';
import { Array, pipe } from 'effect';
import { useCallback, useMemo } from 'react';
import { useFieldArray, useForm } from 'react-hook-form';
import { LuTrash2 } from 'react-icons/lu';

import {
  HeaderCreateResponseSchema,
  HeaderJson,
  HeaderListItemSchema,
  HeaderListResponseSchema,
  HeaderSchema,
  HeaderUpdateRequestSchema,
} from '@the-dev-tools/spec/collection/item/request/v1/request_pb';
import {
  headerCreate,
  headerDelete,
  headerList,
  headerUpdate,
} from '@the-dev-tools/spec/collection/item/request/v1/request-RequestService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { CheckboxRHF } from '@the-dev-tools/ui/checkbox';
import { DataTable } from '@the-dev-tools/ui/data-table';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextFieldRHF } from '@the-dev-tools/ui/text-field';

import { HidePlaceholderCell, useFormTableSync } from './form-table';
import { TextFieldWithVariables } from './variable';

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceIdCan');

interface HeaderTableProps {
  exampleId: Uint8Array;
}

export const HeaderTable = ({ exampleId }: HeaderTableProps) => {
  const queryClient = useQueryClient();

  const { workspaceId } = workspaceRoute.useLoaderData();

  const {
    data: { items },
  } = useConnectSuspenseQuery(headerList, { exampleId });

  const createMutation = useConnectMutation(headerCreate);
  const updateMutation = useConnectMutation(headerUpdate);
  const { mutate: deleteMutate } = useConnectMutation(headerDelete);

  const makeItem = useCallback(
    (headerId?: string, item?: HeaderJson) => ({
      ...item,
      headerId: headerId ?? '',
      enabled: true,
    }),
    [],
  );
  const values = useMemo(
    () => ({
      items: [...items.map((_): HeaderJson => toJson(HeaderListItemSchema, _)), makeItem()],
    }),
    [items, makeItem],
  );
  const { getValues, ...form } = useForm({ values });
  const { remove: removeField, ...fieldArray } = useFieldArray({
    control: form.control,
    name: 'items',
    keyName: 'headerId',
  });

  const columns = useMemo(() => {
    const { accessor, display } = createColumnHelper<HeaderJson>();
    return [
      accessor('enabled', {
        header: '',
        size: 0,
        cell: ({ row, table }) => (
          <HidePlaceholderCell row={row} table={table} className={tw`flex justify-center`}>
            <CheckboxRHF control={form.control} name={`items.${row.index}.enabled`} variant='table-cell' />
          </HidePlaceholderCell>
        ),
      }),
      accessor('key', {
        meta: { divider: false },
        cell: ({ row: { index } }) => (
          <TextFieldWithVariables
            control={form.control}
            name={`items.${index}.key`}
            workspaceId={workspaceId}
            variant='table-cell'
            className='flex-1'
          />
        ),
      }),
      accessor('value', {
        cell: ({ row: { index } }) => (
          <TextFieldWithVariables
            control={form.control}
            name={`items.${index}.value`}
            workspaceId={workspaceId}
            variant='table-cell'
            className='flex-1'
          />
        ),
      }),
      accessor('description', {
        cell: ({ row }) => (
          <TextFieldRHF control={form.control} name={`items.${row.index}.description`} variant='table-cell' />
        ),
      }),
      display({
        id: 'actions',
        header: '',
        size: 0,
        meta: { divider: false },
        cell: ({ row, table }) => (
          <HidePlaceholderCell row={row} table={table}>
            <Button
              className='text-red-700'
              variant='ghost'
              onPress={() => {
                const headerIdJson = getValues(`items.${row.index}.headerId`);
                if (headerIdJson === undefined) return;
                const { headerId } = fromJson(HeaderSchema, {
                  headerId: headerIdJson,
                });
                deleteMutate({ headerId });
                removeField(row.index);
              }}
            >
              <LuTrash2 />
            </Button>
          </HidePlaceholderCell>
        ),
      }),
    ];
  }, [form.control, workspaceId, deleteMutate, getValues, removeField]);

  const table = useReactTable({
    getCoreRowModel: getCoreRowModel(),
    getRowId: (_) => _.headerId ?? '',
    defaultColumn: { minSize: 0 },
    data: fieldArray.fields,
    columns,
  });

  const setData = useCallback(() => {
    const items = pipe(
      getValues('items'),
      Array.dropRight(1),
      Array.map((_) => fromJson(HeaderListItemSchema, _)),
    );
    queryClient.setQueryData(
      createConnectQueryKey({
        schema: headerList,
        cardinality: 'finite',
        input: { exampleId },
      }),
      createProtobufSafeUpdater(headerList, () => create(HeaderListResponseSchema, { items })),
    );
  }, [exampleId, getValues, queryClient]);

  useFormTableSync({
    field: 'items',
    form: { ...form, getValues },
    fieldArray,
    makeItem,
    getRowId: (_) => _.headerId,
    onCreate: async (header) => {
      const response = await createMutation.mutateAsync({
        ...header,
        exampleId,
      });
      return toJson(HeaderCreateResponseSchema, response).headerId ?? '';
    },
    onUpdate: (header) => updateMutation.mutateAsync(fromJson(HeaderUpdateRequestSchema, header)),
    setData,
  });

  return <DataTable table={table} />;
};
