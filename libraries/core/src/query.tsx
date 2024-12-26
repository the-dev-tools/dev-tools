import { create } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { createQueryOptions, useSuspenseQuery as useConnectSuspenseQuery } from '@connectrpc/connect-query';
import { useSuspenseQueries } from '@tanstack/react-query';
import { useRouteContext } from '@tanstack/react-router';
import { getCoreRowModel, useReactTable } from '@tanstack/react-table';
import { Array, HashMap, Option, pipe, Struct } from 'effect';
import { idEqual, Ulid } from 'id128';
import { useMemo } from 'react';
import { useFieldArray, useForm } from 'react-hook-form';

import {
  QueryListItem,
  QueryListItemSchema,
  RequestService,
} from '@the-dev-tools/spec/collection/item/request/v1/request_pb';
import { queryList } from '@the-dev-tools/spec/collection/item/request/v1/request-RequestService_connectquery';
import { DataTable } from '@the-dev-tools/ui/data-table';

import {
  DeltaTableFormItem,
  makeGenericDeltaFormTableColumns,
  makeGenericFormTableColumns,
  TableFormData,
  TableFormItem,
  useFieldArrayTasks,
} from './form-table';

interface QueryTableProps {
  exampleId: Uint8Array;
}

const queryTableColumns = makeGenericFormTableColumns<QueryListItem>();

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

const queryDeltaTableColumns = makeGenericDeltaFormTableColumns<QueryListItem>();

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
