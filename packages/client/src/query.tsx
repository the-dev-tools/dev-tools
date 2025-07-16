import { useRouteContext } from '@tanstack/react-router';
import { Array, Match, Option, pipe, Predicate } from 'effect';
import { Ulid } from 'id128';
import { useDragAndDrop } from 'react-aria-components';
import { QueryDeltaListItem, QueryListItem } from '@the-dev-tools/spec/collection/item/request/v1/request_pb';
import {
  QueryCreateEndpoint,
  QueryDeleteEndpoint,
  QueryDeltaCreateEndpoint,
  QueryDeltaDeleteEndpoint,
  QueryDeltaListEndpoint,
  QueryDeltaResetEndpoint,
  QueryDeltaUpdateEndpoint,
  QueryListEndpoint,
  QueryMoveEndpoint,
  QueryUpdateEndpoint,
} from '@the-dev-tools/spec/meta/collection/item/request/v1/request.endpoints.ts';
import { MovePosition } from '@the-dev-tools/spec/resources/v1/resources_pb';
import { DataTable, useReactTable } from '@the-dev-tools/ui/data-table';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { GenericMessage } from '~api/utils';
import { useQuery } from '~data-client';
import {
  columnActionsCommon,
  columnActionsDeltaCommon,
  columnCheckboxField,
  columnReferenceField,
  columnTextField,
  displayTable,
  makeDeltaItems,
  ReactTableNoMemo,
  useFormTable,
} from './form-table';

interface QueryTableProps {
  deltaExampleId?: Uint8Array | undefined;
  exampleId: Uint8Array;
  isReadOnly?: boolean | undefined;
}

export const QueryTable = ({ deltaExampleId, exampleId, isReadOnly }: QueryTableProps) => {
  if (isReadOnly) return <DisplayTable exampleId={exampleId} />;
  if (deltaExampleId) return <DeltaFormTable deltaExampleId={deltaExampleId} exampleId={exampleId} />;
  return <FormTable exampleId={exampleId} />;
};

const dataColumns = [
  columnCheckboxField<GenericMessage<QueryListItem>>('enabled', { meta: { divider: false } }),
  columnReferenceField<GenericMessage<QueryListItem>>('key', { meta: { isRowHeader: true } }),
  columnReferenceField<GenericMessage<QueryListItem>>('value', { allowFiles: true }),
  columnTextField<GenericMessage<QueryListItem>>('description', { meta: { divider: false } }),
];

interface DisplayTableProps {
  exampleId: Uint8Array;
}

const DisplayTable = ({ exampleId }: DisplayTableProps) => {
  const { items } = useQuery(QueryListEndpoint, { exampleId });

  const table = useReactTable({
    columns: dataColumns,
    data: items,
  });

  return <DataTable {...displayTable<GenericMessage<QueryListItem>>()} table={table} tableAria-label='Query items' />;
};

interface FormTableProps {
  exampleId: Uint8Array;
}

const FormTable = ({ exampleId }: FormTableProps) => {
  const { dataClient } = useRouteContext({ from: '__root__' });

  const items: GenericMessage<QueryListItem>[] = useQuery(QueryListEndpoint, { exampleId }).items;

  const formTable = useFormTable({
    createLabel: 'New param',
    items,
    onCreate: async () => {
      await dataClient.fetch(QueryCreateEndpoint, { enabled: true, exampleId });
      // TODO: improve key matching
      await dataClient.controller.expireAll({ testKey: (_) => _.startsWith(`["${QueryDeltaListEndpoint.name}"`) });
    },
    onUpdate: ({ $typeName: _, ...item }) => dataClient.fetch(QueryUpdateEndpoint, item),
    primaryColumn: 'key',
  });

  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) => [...keys].map((key) => ({ key: key.toString() })),
    onReorder: ({ keys, target: { dropPosition, key } }) =>
      Option.gen(function* () {
        const targetIdCan = yield* Option.liftPredicate(key, Predicate.isString);

        const sourceIdCan = yield* pipe(
          yield* Option.liftPredicate(keys, (_) => _.size === 1),
          Array.fromIterable,
          Array.head,
          Option.filter(Predicate.isString),
        );

        const position = yield* pipe(
          Match.value(dropPosition),
          Match.when('after', () => MovePosition.AFTER),
          Match.when('before', () => MovePosition.BEFORE),
          Match.option,
        );

        void dataClient.fetch(QueryMoveEndpoint, {
          exampleId,
          position,
          queryId: Ulid.fromCanonical(sourceIdCan).bytes,
          targetQueryId: Ulid.fromCanonical(targetIdCan).bytes,
        });
      }),
    renderDropIndicator: () => <tr className={tw`relative z-10 col-span-full h-0 w-full ring ring-violet-700`} />,
  });

  return (
    <ReactTableNoMemo
      columns={[
        ...dataColumns,
        columnActionsCommon<GenericMessage<QueryListItem>>({
          onDelete: (_) => dataClient.fetch(QueryDeleteEndpoint, { queryId: _.queryId }),
        }),
      ]}
      data={items}
      getRowId={(_) => Ulid.construct(_.queryId).toCanonical()}
    >
      {(table) => (
        <DataTable
          {...formTable}
          table={table}
          tableAria-label='Query items'
          tableDragAndDropHooks={dragAndDropHooks}
        />
      )}
    </ReactTableNoMemo>
  );
};

interface DeltaFormTableProps {
  deltaExampleId: Uint8Array;
  exampleId: Uint8Array;
}

const DeltaFormTable = ({ deltaExampleId: exampleId, exampleId: originId }: DeltaFormTableProps) => {
  const { dataClient } = useRouteContext({ from: '__root__' });

  const items = pipe(useQuery(QueryDeltaListEndpoint, { exampleId, originId }).items, (_: QueryDeltaListItem[]) =>
    makeDeltaItems(_, 'queryId'),
  );

  const formTable = useFormTable({
    createLabel: 'New param',
    items,
    onCreate: () => dataClient.fetch(QueryDeltaCreateEndpoint, { enabled: true, exampleId, originId }),
    onUpdate: ({ $typeName: _, ...item }) => dataClient.fetch(QueryDeltaUpdateEndpoint, item),
    primaryColumn: 'key',
  });

  return (
    <ReactTableNoMemo
      columns={[
        ...dataColumns,
        columnActionsDeltaCommon<GenericMessage<QueryDeltaListItem>>({
          onDelete: (_) => dataClient.fetch(QueryDeltaDeleteEndpoint, { queryId: _.queryId }),
          onReset: (_) => dataClient.fetch(QueryDeltaResetEndpoint, { queryId: _.queryId }),
          source: (_) => _.source,
        }),
      ]}
      data={items}
      getRowId={(_) => _.queryId.toString()}
    >
      {(table) => <DataTable {...formTable} table={table} tableAria-label='Query items' />}
    </ReactTableNoMemo>
  );
};
