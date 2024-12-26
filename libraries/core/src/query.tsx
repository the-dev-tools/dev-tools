import { create } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { createQueryOptions, useSuspenseQuery as useConnectSuspenseQuery } from '@connectrpc/connect-query';
import { useSuspenseQueries } from '@tanstack/react-query';
import { getRouteApi, useRouteContext } from '@tanstack/react-router';
import { AccessorKeyColumnDef, createColumnHelper, getCoreRowModel, useReactTable } from '@tanstack/react-table';
import { Array, HashMap, Option, pipe, Struct } from 'effect';
import { idEqual, Ulid } from 'id128';
import { useMemo } from 'react';
import { useFieldArray, useForm, useWatch } from 'react-hook-form';
import { LuTrash2 } from 'react-icons/lu';
import { twJoin } from 'tailwind-merge';

import {
  QueryListItem,
  QueryListItemSchema,
  RequestService,
} from '@the-dev-tools/spec/collection/item/request/v1/request_pb';
import { queryList } from '@the-dev-tools/spec/collection/item/request/v1/request-RequestService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { CheckboxRHF } from '@the-dev-tools/ui/checkbox';
import { DataTable } from '@the-dev-tools/ui/data-table';
import { RedoIcon } from '@the-dev-tools/ui/icons';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextFieldRHF } from '@the-dev-tools/ui/text-field';

import { RHFDevTools } from './dev-tools';
import {
  DeltaTableFormItem,
  HidePlaceholderCell,
  TableFormData,
  TableFormItem,
  useFieldArrayTasks,
} from './form-table';
import { TextFieldWithVariables } from './variable';

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceIdCan');

interface QueryTableProps {
  exampleId: Uint8Array;
}

const queryTableHelper = createColumnHelper<TableFormItem<QueryListItem>>();

const queryTableCommonColumns = [
  queryTableHelper.accessor('data.key', {
    header: 'Key',
    meta: { divider: false },
    cell: ({ table, row: { index } }) => {
      const { workspaceId } = workspaceRoute.useLoaderData();
      return (
        <TextFieldWithVariables
          control={table.options.meta!.control!}
          name={`items.${index}.data.key`}
          workspaceId={workspaceId}
          variant='table-cell'
          className='flex-1'
        />
      );
    },
  }),
  queryTableHelper.accessor('data.value', {
    header: 'Value',
    cell: ({ table, row: { index } }) => {
      const { workspaceId } = workspaceRoute.useLoaderData();
      return (
        <TextFieldWithVariables
          control={table.options.meta!.control!}
          name={`items.${index}.data.value`}
          workspaceId={workspaceId}
          variant='table-cell'
          className='flex-1'
        />
      );
    },
  }),
  queryTableHelper.accessor('data.description', {
    header: 'Description',
    cell: ({ table, row }) => (
      <TextFieldRHF
        control={table.options.meta!.control!}
        name={`items.${row.index}.data.description`}
        variant='table-cell'
      />
    ),
  }),
];

const queryTableColumns = [
  queryTableHelper.accessor('data.enabled', {
    header: ({ table }) => <RHFDevTools control={table.options.meta!.control!} className={tw`size-0`} />,
    size: 0,
    cell: ({ table, row }) => (
      <HidePlaceholderCell row={row} table={table} className={tw`flex justify-center`}>
        <CheckboxRHF
          control={table.options.meta!.control!}
          name={`items.${row.index}.data.enabled`}
          variant='table-cell'
        />
      </HidePlaceholderCell>
    ),
  }),
  ...queryTableCommonColumns,
  queryTableHelper.display({
    id: 'actions',
    header: '',
    size: 0,
    meta: { divider: false },
    cell: ({ table, row }) => (
      <HidePlaceholderCell row={row} table={table}>
        <Button
          className='text-red-700'
          variant='ghost'
          onPress={() => void table.options.meta?.queueTask?.(row.index, 'delete')}
        >
          <LuTrash2 />
        </Button>
      </HidePlaceholderCell>
    ),
  }),
];

export const QueryTable = ({ exampleId }: QueryTableProps) => {
  const { transport } = useRouteContext({ from: '__root__' });
  const requestService = useMemo(() => createClient(RequestService, transport), [transport]);

  const {
    data: { items },
  } = useConnectSuspenseQuery(queryList, { exampleId });

  const emptyItem = (): TableFormItem<QueryListItem> => ({ data: create(QueryListItemSchema, { enabled: true }) });

  const values = useMemo((): TableFormData<TableFormItem<QueryListItem>> => {
    return {
      items: pipe(
        Array.map(items, (_) => ({ id: Ulid.construct(_.queryId).toCanonical(), data: _ })),
        Array.append(emptyItem()),
      ),
    };
  }, [items]);

  const form = useForm({ values });
  const fieldArray = useFieldArray({ control: form.control, name: 'items' });

  const { queueTask, itemTransaction } = useFieldArrayTasks({
    form,
    itemPath: (index) => `items.${index}`,
    itemKey: (_) => Ulid.construct(_.data.queryId).toCanonical(),
    onTask: async ({ index, item: { data }, type }) => {
      if (type === 'change' && data.queryId.length === 0) {
        const { queryId } = await requestService.queryCreate({ ...Struct.omit(data, '$typeName'), exampleId });
        const newIdCan = Ulid.construct(queryId).toCanonical();
        itemTransaction(newIdCan, () => {
          form.setValue(`items.${index}.data.queryId`, queryId);
          fieldArray.append(emptyItem());
        });
      } else if (type === 'change') {
        await requestService.queryUpdate(Struct.omit(data, '$typeName'));
      } else if (type === 'delete') {
        await requestService.queryDelete({ queryId: data.queryId });
        itemTransaction(Ulid.construct(data.queryId).toCanonical(), () => void fieldArray.remove(index));
      }
    },
  });

  const table = useReactTable({
    getCoreRowModel: getCoreRowModel(),
    getRowId: (_) => (_ as (typeof fieldArray.fields)[number]).id,
    defaultColumn: { minSize: 0 },
    data: fieldArray.fields,
    meta: { queueTask, control: form.control },
    columns: queryTableColumns,
  });

  return <DataTable table={table} />;
};

const queryDeltaTableHelper = createColumnHelper<DeltaTableFormItem<QueryListItem>>();
const queryDeltaTableColumns = [
  queryDeltaTableHelper.accessor('data.enabled', {
    header: ({ table }) => <RHFDevTools control={table.options.meta!.control!} className={tw`size-0`} />,
    size: 0,
    cell: ({ table, row }) => (
      <div className={tw`flex justify-center`}>
        <CheckboxRHF
          control={table.options.meta!.control!}
          name={`items.${row.index}.data.enabled`}
          variant='table-cell'
        />
      </div>
    ),
  }),
  ...(queryTableCommonColumns as AccessorKeyColumnDef<DeltaTableFormItem<QueryListItem>>[]),
  queryDeltaTableHelper.display({
    id: 'actions',
    header: '',
    size: 0,
    meta: { divider: false },
    cell: function ActionCell({ table, row }) {
      const [parentId, itemId] = useWatch({
        control: table.options.meta!.control!,
        name: [`items.${row.index}.parentData.queryId`, `items.${row.index}.data.queryId`],
      });

      const parentUlid = Ulid.construct(parentId);
      const itemUlid = Ulid.construct(itemId);

      return (
        <Button
          className={twJoin(tw`text-slate-500`, idEqual(parentUlid, itemUlid) && tw`invisible`)}
          variant='ghost'
          onPress={() => void table.options.meta!.queueTask!(row.index, 'undo')}
        >
          <RedoIcon />
        </Button>
      );
    },
  }),
];

interface QueryDeltaTableProps extends QueryTableProps {
  deltaExampleId: Uint8Array;
}

export const QueryDeltaTable = ({ exampleId, deltaExampleId }: QueryDeltaTableProps) => {
  const { transport } = useRouteContext({ from: '__root__' });
  const requestService = useMemo(() => createClient(RequestService, transport), [transport]);

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

    const items = baseItems.map(
      (_): DeltaTableFormItem<QueryListItem> => ({
        parentData: _,
        data: Option.getOrElse(HashMap.get(deltaItemMap, _.queryId), () => _),
      }),
    );

    return { items };
  }, [deltaItems, baseItems]);

  const form = useForm({ values });
  const fieldArray = useFieldArray({ control: form.control, name: 'items' });

  const { queueTask, itemTransaction } = useFieldArrayTasks({
    form,
    itemPath: (index) => `items.${index}`,
    itemKey: (_) => Ulid.construct(_.data.queryId).toCanonical(),
    onTask: async ({ index, item, type }) => {
      const { parentData, data } = item;

      const parentUlid = Ulid.construct(parentData.queryId);
      const itemUlid = Ulid.construct(data.queryId);

      if (type === 'change' && idEqual(parentUlid, itemUlid)) {
        const { queryId } = await requestService.queryCreate({ ...Struct.omit(data, '$typeName'), exampleId });
        const newIdCan = Ulid.construct(queryId).toCanonical();
        itemTransaction(newIdCan, () => void form.setValue(`items.${index}.data.queryId`, queryId));
      } else if (type === 'change') {
        await requestService.queryUpdate(Struct.omit(data, '$typeName'));
      } else if (type === 'undo') {
        await requestService.queryDelete({ queryId: data.queryId });
        itemTransaction(parentUlid.toCanonical(), () => void form.setValue(`items.${index}.data`, parentData));
      }
    },
  });

  const table = useReactTable({
    getCoreRowModel: getCoreRowModel(),
    getRowId: (_) => (_ as (typeof fieldArray.fields)[number]).id,
    defaultColumn: { minSize: 0 },
    data: fieldArray.fields,
    meta: { queueTask, control: form.control },
    columns: queryDeltaTableColumns,
  });

  return <DataTable table={table} />;
};
