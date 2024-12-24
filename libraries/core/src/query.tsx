import { create, fromJson, toJson } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import {
  createConnectQueryKey,
  createProtobufSafeUpdater,
  createQueryOptions,
  useMutation as useConnectMutation,
  useSuspenseQuery as useConnectSuspenseQuery,
} from '@connectrpc/connect-query';
import { useQueryClient, useSuspenseQueries } from '@tanstack/react-query';
import { getRouteApi, useRouteContext } from '@tanstack/react-router';
import { createColumnHelper, getCoreRowModel, useReactTable } from '@tanstack/react-table';
import { Array, HashMap, Option, pipe, Struct } from 'effect';
import { idEqual, Ulid } from 'id128';
import { useCallback, useMemo, useRef } from 'react';
import { useFieldArray, useForm } from 'react-hook-form';
import { LuTrash2 } from 'react-icons/lu';
import { twJoin } from 'tailwind-merge';

import {
  QueryCreateResponseSchema,
  QueryJson,
  QueryListItemSchema,
  QueryListResponseSchema,
  QuerySchema,
  QueryUpdateRequestSchema,
  RequestService,
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
import { RedoIcon } from '@the-dev-tools/ui/icons';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextFieldRHF } from '@the-dev-tools/ui/text-field';

import { RHFDevTools } from './dev-tools';
import { FormWatch, HidePlaceholderCell, useFormTableSync } from './form-table';
import { TextFieldWithVariables } from './variable';

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceIdCan');

interface QueryTableProps {
  exampleId: Uint8Array;
}

export const QueryTable = ({ exampleId }: QueryTableProps) => {
  const queryClient = useQueryClient();

  const { workspaceId } = workspaceRoute.useLoaderData();

  const {
    data: { items },
  } = useConnectSuspenseQuery(queryList, { exampleId });

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
        input: { exampleId },
      }),
      createProtobufSafeUpdater(queryList, () => create(QueryListResponseSchema, { items })),
    );
  }, [exampleId, getValues, queryClient]);

  const columns = useMemo(() => {
    const { accessor, display } = createColumnHelper<QueryJson>();
    return [
      accessor('enabled', {
        header: () => <RHFDevTools control={form.control} className={tw`size-0`} />,
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

interface QueryDeltaTableProps {
  exampleId: Uint8Array;
  deltaExampleId: Uint8Array;
}

export const QueryDeltaTable = ({ exampleId, deltaExampleId }: QueryDeltaTableProps) => {
  const { transport } = useRouteContext({ from: '__root__' });
  const requestService = useMemo(() => createClient(RequestService, transport), [transport]);

  const { workspaceId } = workspaceRoute.useLoaderData();

  const [
    {
      data: { items: baseItems },
    },
    {
      data: { items: deltaItems },
    },
  ] = useSuspenseQueries({
    queries: [
      createQueryOptions(queryList, { exampleId }, { transport }),
      createQueryOptions(queryList, { exampleId: deltaExampleId }, { transport }),
    ],
  });

  const values = useMemo(() => {
    const deltaItemMap = pipe(
      deltaItems.map((_) => [_.parentQueryId, _] as const),
      HashMap.fromIterable,
    );

    const items = baseItems.map((_) => ({
      baseIdCan: Ulid.construct(_.queryId).toCanonical(),
      baseId: _.queryId,
      baseValue: _,
      value: Option.getOrElse(HashMap.get(deltaItemMap, _.queryId), () => _),
    }));

    return { items };
  }, [deltaItems, baseItems]);

  type Item = (typeof values)['items'][number];

  const { mutate: deleteMutate } = useConnectMutation(queryDelete);

  const { getValues, ...form } = useForm({ values });
  const fieldArray = useFieldArray({ control: form.control, name: 'items' });

  const dirtyRef = useRef(new Map<string, Item>());

  const columns = useMemo(() => {
    const { accessor, display } = createColumnHelper<Item>();
    return [
      accessor('value.enabled', {
        header: () => <RHFDevTools control={form.control} className={tw`size-0`} />,
        size: 0,
        cell: ({ row }) => (
          <div className={tw`flex justify-center`}>
            <CheckboxRHF control={form.control} name={`items.${row.index}.value.enabled`} variant='table-cell' />
          </div>
        ),
      }),
      accessor('value.key', {
        header: 'Key',
        meta: { divider: false },
        cell: ({ row: { index } }) => (
          <TextFieldWithVariables
            control={form.control}
            name={`items.${index}.value.key`}
            workspaceId={workspaceId}
            variant='table-cell'
            className='flex-1'
          />
        ),
      }),
      accessor('value.value', {
        header: 'Value',
        cell: ({ row: { index } }) => (
          <TextFieldWithVariables
            control={form.control}
            name={`items.${index}.value.value`}
            workspaceId={workspaceId}
            variant='table-cell'
            className='flex-1'
          />
        ),
      }),
      accessor('value.description', {
        header: 'Description',
        cell: ({ row }) => (
          <TextFieldRHF control={form.control} name={`items.${row.index}.value.description`} variant='table-cell' />
        ),
      }),
      display({
        id: 'actions',
        header: '',
        size: 0,
        meta: { divider: false },
        cell: ({ row }) => (
          <FormWatch
            control={form.control}
            name={[`items.${row.index}.baseIdCan`, `items.${row.index}.baseId`, `items.${row.index}.value.queryId`]}
          >
            {([baseIdCan, baseId, itemId]) => {
              const baseUlid = Ulid.construct(baseId);
              const itemUlid = Ulid.construct(itemId);

              return (
                <Button
                  className={twJoin(tw`text-slate-500`, idEqual(baseUlid, itemUlid) && tw`invisible`)}
                  variant='ghost'
                  onPress={() => {
                    const [deltaQueryId, baseQuery] = getValues([
                      `items.${row.index}.value.queryId`,
                      `items.${row.index}.baseValue`,
                    ]);
                    deleteMutate({ queryId: deltaQueryId });
                    form.setValue(`items.${row.index}.value`, baseQuery);
                    dirtyRef.current.delete(baseIdCan);
                  }}
                >
                  <RedoIcon />
                </Button>
              );
            }}
          </FormWatch>
        ),
      }),
    ];
  }, [form, workspaceId, getValues, deleteMutate]);

  const table = useReactTable({
    getCoreRowModel: getCoreRowModel(),
    getRowId: (_) => _.baseIdCan,
    defaultColumn: { minSize: 0 },
    data: fieldArray.fields,
    columns,
  });

  const onUpdate = useCallback(
    async ({ baseIdCan, baseId, value }: Item) => {
      const baseUlid = Ulid.construct(baseId);
      const itemUlid = Ulid.construct(value.queryId);

      if (idEqual(baseUlid, itemUlid)) {
        const { queryId } = await requestService.queryCreate({ ...Struct.omit(value, '$typeName'), exampleId });
        const index = getValues('items').findIndex((_) => _.baseIdCan === baseIdCan);
        form.setValue(`items.${index}.value.queryId`, queryId);
        return baseIdCan;
      }

      await requestService.queryUpdate(Struct.omit(value, '$typeName'));
      return;
    },
    [exampleId, form, getValues, requestService],
  );

  useFormTableSync({
    field: 'items',
    form: { ...form, getValues },
    fieldArray,
    dirtyRef,
    getRowId: (_) => _.baseIdCan,
    onUpdate,
  });

  return <DataTable table={table} />;
};
