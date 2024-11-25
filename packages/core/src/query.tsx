import { create, fromJson, toJson } from '@bufbuild/protobuf';
import {
  createConnectQueryKey,
  createProtobufSafeUpdater,
  useMutation as useConnectMutation,
  useQuery as useConnectQuery,
} from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';
import { createFileRoute, getRouteApi } from '@tanstack/react-router';
import { createColumnHelper, getCoreRowModel, useReactTable } from '@tanstack/react-table';
import { Array, pipe } from 'effect';
import { useCallback, useMemo } from 'react';
import { useFieldArray, useForm } from 'react-hook-form';
import { LuTrash2 } from 'react-icons/lu';

import {
  QueryCreateResponseSchema,
  QueryJson,
  QueryListItem,
  QueryListItemSchema,
  QueryListResponseSchema,
  QuerySchema,
  QueryUpdateRequestSchema,
} from '@the-dev-tools/spec/collection/item/request/v1/request_pb';
import {
  queryCreate,
  queryDelete,
  queryList,
  queryUpdate,
} from '@the-dev-tools/spec/collection/item/request/v1/request-RequestService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { CheckboxRHF } from '@the-dev-tools/ui/checkbox';
import { DataTable } from '@the-dev-tools/ui/data-table';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextFieldRHF } from '@the-dev-tools/ui/text-field';

import { HidePlaceholderCell, useFormTableSync } from './form-table';
import { TextFieldWithVariables } from './variable';

export const Route = createFileRoute(
  '/_authorized/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan/',
)({
  component: Tab,
});

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceIdCan');
const endpointRoute = getRouteApi(
  '/_authorized/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan',
);

function Tab() {
  const { exampleId } = endpointRoute.useLoaderData();
  const query = useConnectQuery(queryList, { exampleId });
  if (!query.isSuccess) return null;
  return <Table items={query.data.items} />;
}

interface TableProps {
  items: QueryListItem[];
}

const Table = ({ items }: TableProps) => {
  const queryClient = useQueryClient();

  const { workspaceId } = workspaceRoute.useLoaderData();
  const { exampleId } = endpointRoute.useLoaderData();

  const createMutation = useConnectMutation(queryCreate);
  const updateMutation = useConnectMutation(queryUpdate);
  const { mutate: deleteMutate } = useConnectMutation(queryDelete);

  const makeItem = useCallback(
    (queryId?: string, item?: QueryJson) => ({
      ...item,
      queryId: queryId ?? '',
      enabled: true,
    }),
    [],
  );
  const values = useMemo(
    () => ({
      items: [...items.map((_): QueryJson => toJson(QueryListItemSchema, _)), makeItem()],
    }),
    [items, makeItem],
  );
  const { getValues, ...form } = useForm({ values });
  const { remove: removeField, ...fieldArray } = useFieldArray({
    control: form.control,
    name: 'items',
    keyName: 'queryId',
  });

  const setData = useCallback(() => {
    const items = pipe(
      getValues('items'),
      Array.dropRight(1),
      Array.map((_) => fromJson(QueryListItemSchema, _)),
    );
    queryClient.setQueryData(
      createConnectQueryKey({
        schema: queryList,
        cardinality: 'finite',
        input: { items },
      }),
      createProtobufSafeUpdater(queryList, () => create(QueryListResponseSchema, { items })),
    );
  }, [getValues, queryClient]);

  const columns = useMemo(() => {
    const { accessor, display } = createColumnHelper<QueryJson>();
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
                const queryIdJson = getValues(`items.${row.index}.queryId`);
                if (queryIdJson === undefined) return;
                const { queryId } = fromJson(QuerySchema, {
                  queryId: queryIdJson,
                });
                deleteMutate({ queryId });
                removeField(row.index);
                void setData();
              }}
            >
              <LuTrash2 />
            </Button>
          </HidePlaceholderCell>
        ),
      }),
    ];
  }, [form.control, workspaceId, deleteMutate, getValues, removeField, setData]);

  const table = useReactTable<QueryJson>({
    getCoreRowModel: getCoreRowModel(),
    getRowId: (_) => _.queryId ?? '',
    defaultColumn: { minSize: 0 },
    data: fieldArray.fields,
    columns,
  });

  useFormTableSync({
    field: 'items',
    form: { ...form, getValues },
    fieldArray,
    makeItem,
    getRowId: (_) => _.queryId,
    onCreate: async (query) => {
      const response = await createMutation.mutateAsync({
        ...query,
        exampleId,
      });
      return toJson(QueryCreateResponseSchema, response).queryId ?? '';
    },
    onUpdate: (query) => updateMutation.mutateAsync(fromJson(QueryUpdateRequestSchema, query)),
    onChange: setData,
    setData,
  });

  return <DataTable table={table} />;
};
