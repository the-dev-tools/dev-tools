import { create, toJson } from '@bufbuild/protobuf';
import { createConnectQueryKey, createProtobufSafeUpdater, createQueryOptions } from '@connectrpc/connect-query';
import { makeUrl } from '@effect/platform/UrlParams';
import { effectTsResolver } from '@hookform/resolvers/effect-ts';
import { useQuery, useQueryClient, useSuspenseQueries } from '@tanstack/react-query';
import { createFileRoute, getRouteApi, redirect, useRouteContext } from '@tanstack/react-router';
import { createColumnHelper, getCoreRowModel, useReactTable } from '@tanstack/react-table';
import CodeMirror from '@uiw/react-codemirror';
import { Array, Duration, Either, HashMap, Match, MutableHashMap, Option, pipe, Schema, Struct } from 'effect';
import { Ulid } from 'id128';
import { format as prettierFormat } from 'prettier/standalone';
import { Fragment, Suspense, useMemo, useState } from 'react';
import { Collection, Dialog, DialogTrigger, MenuTrigger, Tab, TabList, TabPanel, Tabs } from 'react-aria-components';
import { useForm } from 'react-hook-form';
import { FiClock, FiMoreHorizontal, FiSave } from 'react-icons/fi';
import { Panel, PanelGroup } from 'react-resizable-panels';
import { twJoin, twMerge } from 'tailwind-merge';

import { useConnectMutation, useConnectSuspenseQuery } from '@the-dev-tools/api/connect-query';
import {
  endpointGet,
  endpointUpdate,
} from '@the-dev-tools/spec/collection/item/endpoint/v1/endpoint-EndpointService_connectquery';
import { ExampleVersionsItem } from '@the-dev-tools/spec/collection/item/example/v1/example_pb';
import {
  exampleCreate,
  exampleDelete,
  exampleGet,
  exampleRun,
  exampleUpdate,
  exampleVersions,
} from '@the-dev-tools/spec/collection/item/example/v1/example-ExampleService_connectquery';
import {
  QueryCreateRequest,
  QueryCreateRequestSchema,
  QueryListItemSchema,
  QueryListResponseSchema,
  QueryUpdateRequest,
  QueryUpdateRequestSchema,
} from '@the-dev-tools/spec/collection/item/request/v1/request_pb';
import {
  queryCreate,
  queryList,
  queryUpdate,
} from '@the-dev-tools/spec/collection/item/request/v1/request-RequestService_connectquery';
import { ResponseHeaderListItem } from '@the-dev-tools/spec/collection/item/response/v1/response_pb';
import {
  responseAssertList,
  responseGet,
  responseHeaderList,
} from '@the-dev-tools/spec/collection/item/response/v1/response-ResponseService_connectquery';
import { ReferenceKeySchema } from '@the-dev-tools/spec/reference/v1/reference_pb';
import { Button } from '@the-dev-tools/ui/button';
import { DataTable } from '@the-dev-tools/ui/data-table';
import { Spinner } from '@the-dev-tools/ui/icons';
import { ListBoxItem } from '@the-dev-tools/ui/list-box';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { MethodBadge } from '@the-dev-tools/ui/method-badge';
import { Modal } from '@the-dev-tools/ui/modal';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { Select, SelectRHF } from '@the-dev-tools/ui/select';
import { Separator } from '@the-dev-tools/ui/separator';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField, TextFieldRHF, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { formatSize } from '@the-dev-tools/utils/helpers';

import { AssertionView } from './assertions';
import { BodyView } from './body';
import { HeaderTable } from './headers';
import { QueryTable } from './query';
import { ReferenceContext } from './reference';
import { StatusBar } from './status-bar';

export class EndpointRouteSearch extends Schema.Class<EndpointRouteSearch>('EndpointRouteSearch')({
  responseIdCan: pipe(Schema.String, Schema.optional),
}) {}

export const Route = createFileRoute(
  '/_authorized/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan',
)({
  component: Page,
  shouldReload: false,
  validateSearch: (_) => Schema.decodeSync(EndpointRouteSearch)(_),
  loaderDeps: (_) => Struct.pick(_.search, 'responseIdCan'),
  loader: async ({
    params: { workspaceIdCan, endpointIdCan, exampleIdCan },
    deps: { responseIdCan },
    context: { transport, queryClient },
  }) => {
    const endpointId = Ulid.fromCanonical(endpointIdCan).bytes;
    const exampleId = Ulid.fromCanonical(exampleIdCan).bytes;
    const responseId = pipe(
      Option.fromNullable(responseIdCan),
      Option.map((_) => Ulid.fromCanonical(_).bytes),
    );

    try {
      await Promise.all([
        queryClient.ensureQueryData(createQueryOptions(exampleGet, { exampleId }, { transport })),
        queryClient.ensureQueryData(createQueryOptions(endpointGet, { endpointId }, { transport })),
        queryClient.ensureQueryData(createQueryOptions(queryList, { exampleId }, { transport })),
        ...pipe(
          Option.map(responseId, (_) =>
            queryClient.ensureQueryData(createQueryOptions(responseGet, { responseId: _ }, { transport })),
          ),
          Option.toArray,
        ),
      ]);
    } catch {
      redirect({
        to: '/workspace/$workspaceIdCan',
        params: { workspaceIdCan },
        throw: true,
      });
    }

    return { endpointId, exampleId, responseId };
  },
});

function Page() {
  const { endpointId, exampleId } = Route.useLoaderData();

  const { workspaceId } = workspaceRoute.useLoaderData();

  const { data: example } = useConnectSuspenseQuery(exampleGet, { exampleId });

  return (
    <Panel id='main' order={2}>
      <PanelGroup direction='vertical'>
        <Panel id='request' order={1} className='flex h-full flex-col'>
          <EndpointForm endpointId={endpointId} exampleId={exampleId} />

          <ReferenceContext value={{ exampleId, workspaceId }}>
            <EndpointRequestView exampleId={exampleId} />
          </ReferenceContext>
        </Panel>
        {example.lastResponseId && (
          <>
            <PanelResizeHandle direction='vertical' />
            <Panel id='response' order={2} defaultSize={40}>
              <ResponsePanel responseId={example.lastResponseId} fullWidth />
            </Panel>
          </>
        )}
        <StatusBar />
      </PanelGroup>
    </Panel>
  );
}

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceIdCan');

interface EndpointRequestViewProps {
  exampleId: Uint8Array;
  deltaExampleId?: Uint8Array;
  isReadOnly?: boolean;
  className?: string;
}

export const EndpointRequestView = ({ exampleId, deltaExampleId, isReadOnly, className }: EndpointRequestViewProps) => (
  <Tabs className={twMerge(tw`flex flex-1 flex-col gap-6 overflow-auto p-6 pt-4`, className)}>
    <TabList className={tw`flex gap-3 border-b border-slate-200`}>
      <Tab
        id='params'
        className={({ isSelected }) =>
          twMerge(
            tw`-mb-px cursor-pointer border-b-2 border-transparent py-1.5 text-md font-medium leading-5 tracking-tight text-slate-500 transition-colors`,
            isSelected && tw`border-b-violet-700 text-slate-800`,
          )
        }
      >
        Params
      </Tab>

      <Tab
        id='headers'
        className={({ isSelected }) =>
          twMerge(
            tw`-mb-px cursor-pointer border-b-2 border-transparent py-1.5 text-md font-medium leading-5 tracking-tight text-slate-500 transition-colors`,
            isSelected && tw`border-b-violet-700 text-slate-800`,
          )
        }
      >
        Headers
      </Tab>

      <Tab
        id='body'
        className={({ isSelected }) =>
          twMerge(
            tw`-mb-px cursor-pointer border-b-2 border-transparent py-1.5 text-md font-medium leading-5 tracking-tight text-slate-500 transition-colors`,
            isSelected && tw`border-b-violet-700 text-slate-800`,
          )
        }
      >
        Body
      </Tab>

      <Tab
        id='assertions'
        className={({ isSelected }) =>
          twMerge(
            tw`-mb-px cursor-pointer border-b-2 border-transparent py-1.5 text-md font-medium leading-5 tracking-tight text-slate-500 transition-colors`,
            isSelected && tw`border-b-violet-700 text-slate-800`,
          )
        }
      >
        Assertion
      </Tab>
    </TabList>

    <Suspense
      fallback={
        <div className={tw`flex h-full items-center justify-center`}>
          <Spinner className={tw`size-12`} />
        </div>
      }
    >
      <TabPanel id='params'>
        <QueryTable exampleId={exampleId} deltaExampleId={deltaExampleId} isReadOnly={isReadOnly} />
      </TabPanel>

      <TabPanel id='headers'>
        <HeaderTable exampleId={exampleId} deltaExampleId={deltaExampleId} isReadOnly={isReadOnly} />
      </TabPanel>

      <TabPanel id='body'>
        <BodyView exampleId={exampleId} deltaExampleId={deltaExampleId} isReadOnly={isReadOnly} />
      </TabPanel>

      <TabPanel id='assertions'>
        <AssertionView exampleId={exampleId} isReadOnly={isReadOnly} />
      </TabPanel>
    </Suspense>
  </Tabs>
);

const methods = ['GET', 'HEAD', 'POST', 'PUT', 'DELETE', 'CONNECT', 'OPTION', 'TRACE', 'PATCH'] as const;

interface UseEndpointUrlProps {
  endpointId: Uint8Array;
  exampleId: Uint8Array;
}

export const useEndpointUrl = ({ endpointId, exampleId }: UseEndpointUrlProps) => {
  const { transport } = useRouteContext({ from: '__root__' });

  const [
    { data: endpoint },
    {
      data: { items: queries },
    },
  ] = useSuspenseQueries({
    queries: [
      createQueryOptions(endpointGet, { endpointId }, { transport }),
      createQueryOptions(queryList, { exampleId }, { transport }),
    ],
  });

  return useMemo(() => {
    return pipe(
      Option.fromNullable(endpoint.url || null),
      Option.flatMap((url) =>
        pipe(
          Array.filterMap(queries, (_) => {
            if (!_.enabled) return Option.none();
            else return Option.some([_.key, _.value] as const);
          }),
          (_) => makeUrl(url, _, Option.none()),
          Either.getRight,
        ),
      ),
      Option.map((_) => _.toString()),
      Option.getOrElse(() => endpoint.url),
    );
  }, [endpoint.url, queries]);
};

class EndpointFormData extends Schema.Class<EndpointFormData>('EndpointFormData')({
  method: Schema.String,
  url: Schema.String,
}) {}

interface EndpointFormProps {
  endpointId: Uint8Array;
  exampleId: Uint8Array;
}

export const EndpointForm = ({ endpointId, exampleId }: EndpointFormProps) => {
  const { transport } = Route.useRouteContext();

  const [
    { data: endpoint },
    { data: example },
    {
      data: { items: queries },
    },
  ] = useSuspenseQueries({
    queries: [
      createQueryOptions(endpointGet, { endpointId }, { transport }),
      createQueryOptions(exampleGet, { exampleId }, { transport }),
      createQueryOptions(queryList, { exampleId }, { transport }),
    ],
  });

  const queryClient = useQueryClient();

  const endpointUpdateMutation = useConnectMutation(endpointUpdate);
  const exampleUpdateMutation = useConnectMutation(exampleUpdate);
  const exampleCreateMutation = useConnectMutation(exampleCreate);
  const exampleDeleteMutation = useConnectMutation(exampleDelete);
  const exampleRunMutation = useConnectMutation(exampleRun);

  const queryUpdateMutation = useConnectMutation(queryUpdate);
  const queryCreateMutation = useConnectMutation(queryCreate);

  const url = useEndpointUrl({ endpointId, exampleId });

  const values = useMemo(() => {
    return new EndpointFormData({
      url,
      method: Array.contains(methods, endpoint.method) ? endpoint.method : 'N/A',
    });
  }, [endpoint.method, url]);

  const form = useForm({
    resolver: effectTsResolver(EndpointFormData),
    values,
  });

  const onSubmit = form.handleSubmit(async ({ method, url: urlString }) => {
    const { origin = '', pathname = '', searchParams = new URLSearchParams() } = !urlString ? {} : new URL(urlString);

    endpointUpdateMutation.mutate({ endpointId, method, url: origin + pathname });

    const queryMap = pipe(
      searchParams.entries(),
      Array.fromIterable,
      Array.map(([key, value]): [string, QueryCreateRequest | QueryUpdateRequest] => [
        key + value,
        create(QueryCreateRequestSchema, { key, value }),
      ]),
      MutableHashMap.fromIterable,
    );

    queries.forEach(({ queryId, key, value, enabled }) => {
      MutableHashMap.modifyAt(
        queryMap,
        key + value,
        Option.match({
          onSome: () => {
            if (enabled) return Option.none();
            return Option.some(create(QueryUpdateRequestSchema, { queryId, enabled: true }));
          },
          onNone: () => {
            if (!enabled) return Option.none();
            return Option.some(create(QueryUpdateRequestSchema, { queryId, enabled: false }));
          },
        }),
      );
    });

    const queryIdIndexMap = pipe(
      queries,
      Array.map(({ queryId }, index) => [Ulid.construct(queryId).toRaw(), index] as const),
      HashMap.fromIterable,
    );

    const newQueryList = Array.copy(queries);
    await pipe(
      Array.fromIterable(queryMap),
      Array.map(async ([_, query]) => {
        if (query.$typeName === 'collection.item.request.v1.QueryUpdateRequest') {
          await queryUpdateMutation.mutateAsync(query);
          const index = HashMap.unsafeGet(queryIdIndexMap, Ulid.construct(query.queryId).toRaw());
          const oldQuery = newQueryList[index];
          if (!oldQuery) return;
          newQueryList[index] = create(QueryListItemSchema, {
            ...oldQuery,
            ...Struct.omit(query, '$typeName'),
          });
        } else {
          const { queryId } = await queryCreateMutation.mutateAsync(query);
          newQueryList.push(
            create(QueryListItemSchema, {
              ...Struct.omit(query, '$typeName'),
              queryId,
            }),
          );
        }
      }),
      (_) => Promise.allSettled(_),
    );

    queryClient.setQueryData(
      createConnectQueryKey({
        schema: queryList,
        cardinality: 'finite',
        input: { exampleId },
      }),
      createProtobufSafeUpdater(queryList, () => create(QueryListResponseSchema, { items: newQueryList })),
    );
  });

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    value: example.name,
    onSuccess: (_) => exampleUpdateMutation.mutateAsync({ exampleId, name: _ }),
  });

  return (
    <form onSubmit={onSubmit}>
      <div className='flex items-center gap-2 border-b border-slate-200 px-4 py-2.5'>
        <div className={tw`flex flex-1 select-none gap-1 text-md font-medium leading-5 tracking-tight text-slate-400`}>
          {example.breadcrumbs.map((_, index) => (
            <Fragment key={`${index} ${_}`}>
              <span>{_}</span>
              <span>/</span>
            </Fragment>
          ))}

          {isEditing ? (
            <TextField
              inputClassName={tw`-my-1 py-1 leading-none text-slate-800`}
              isDisabled={exampleUpdateMutation.isPending}
              {...textFieldProps}
            />
          ) : (
            <h2 className={tw`cursor-pointer text-slate-800`} onContextMenu={onContextMenu}>
              {example.name}
            </h2>
          )}
        </div>

        <DialogTrigger>
          <Button variant='ghost' className={tw`px-2 py-1 text-slate-800`}>
            <FiClock className={tw`size-4 text-slate-500`} /> Response History
          </Button>

          <HistoryModal exampleId={exampleId} />
        </DialogTrigger>

        {/* TODO: implement copy link */}
        {/* <Button variant='ghost' className={tw`px-2 py-1 text-slate-800`}>
          <FiLink className={tw`size-4 text-slate-500`} /> Copy Link
        </Button> */}

        {/* <Separator orientation='vertical' className={tw`h-4`} /> */}

        <Button type='submit' variant='ghost' className={tw`px-2 py-1 text-slate-800`}>
          <FiSave className={tw`size-4 text-slate-500`} /> Save
        </Button>

        <MenuTrigger {...menuTriggerProps}>
          <Button variant='ghost' className={tw`p-1`}>
            <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
          </Button>

          <Menu {...menuProps}>
            <MenuItem onAction={() => void exampleCreateMutation.mutate({ endpointId, name: 'New Example' })}>
              Add example
            </MenuItem>

            <Separator />

            <MenuItem onAction={() => void edit()}>Rename</MenuItem>

            <MenuItem variant='danger' onAction={() => void exampleDeleteMutation.mutate({ exampleId })}>
              Delete
            </MenuItem>
          </Menu>
        </MenuTrigger>
      </div>

      <div className={tw`flex gap-3 p-6 pb-0`}>
        <div className='flex flex-1 items-center gap-3 rounded-lg border border-slate-300 px-3 py-2 shadow-sm'>
          <SelectRHF control={form.control} name='method' aria-label='Method' triggerClassName={tw`border-none p-0`}>
            {methods.map((_) => (
              <ListBoxItem key={_} id={_} textValue={_}>
                <MethodBadge method={_} size='lg' />
              </ListBoxItem>
            ))}
          </SelectRHF>

          <Separator orientation='vertical' className={tw`h-7`} />

          <TextFieldRHF
            control={form.control}
            onBlur={onSubmit}
            name='url'
            aria-label='URL'
            className={tw`flex-1`}
            inputClassName={tw`border-none font-medium tracking-tight`}
          />
        </div>

        <Button
          variant='primary'
          className={tw`px-6`}
          onPress={async () => {
            await onSubmit();
            const { responseId } = await exampleRunMutation.mutateAsync({
              exampleId,
            });
            // TODO: remove manual update once optional field normalization is fixed
            queryClient.setQueryData(
              createConnectQueryKey({
                schema: exampleGet,
                transport,
                cardinality: 'finite',
                input: { exampleId },
              }),
              createProtobufSafeUpdater(exampleGet, (old) => {
                if (old === undefined) return undefined;
                return { ...old, lastResponseId: responseId };
              }),
            );
          }}
        >
          Send
        </Button>
      </div>
    </form>
  );
};

class HistoryTabId extends Schema.Class<HistoryTabId>('HistoryTabId')({
  endpointId: Schema.Uint8Array,
  exampleId: Schema.Uint8Array,
}) {}

interface HistoryModalProps {
  exampleId: Uint8Array;
}

const HistoryModal = ({ exampleId }: HistoryModalProps) => {
  const {
    data: { items: versions },
  } = useConnectSuspenseQuery(exampleVersions, { exampleId });

  return (
    <Modal modalSize='lg' isDismissable>
      <Dialog className={tw`size-full outline-none`}>
        <Tabs className={tw`flex h-full`} orientation='vertical'>
          <div className={tw`flex w-64 flex-col border-r border-slate-200 bg-slate-50 p-4 tracking-tight`}>
            <div className={tw`mb-4`}>
              <div className={tw`mb-0.5 text-sm font-semibold leading-5 text-slate-800`}>Response History</div>
              <div className={tw`text-xs leading-4 text-slate-500`}>History of your API response</div>
            </div>
            <div className={tw`grid grid-cols-[auto_1fr] gap-x-0.5`}>
              <div className={tw`flex flex-col items-center gap-0.5`}>
                <div className={tw`flex-1`} />
                <div className={tw`size-2 rounded-full border border-violet-700 p-px`}>
                  <div className={tw`size-full rounded-full border border-inherit`} />
                </div>
                <div className={tw`w-px flex-1 bg-slate-200`} />
              </div>

              <div className={tw`p-2 text-md font-semibold leading-5 tracking-tight text-violet-700`}>
                Current Version
              </div>

              <div className={tw`flex flex-col items-center gap-0.5`}>
                <div className={tw`w-px flex-1 bg-slate-200`} />
                <div className={tw`size-2 rounded-full bg-slate-300`} />
                <div className={tw`w-px flex-1 bg-slate-200`} />
              </div>

              <div className={tw`p-2 text-md font-semibold leading-5 tracking-tight text-slate-800`}>
                {versions.length} previous responses
              </div>

              <div className={tw`mb-2 w-px flex-1 justify-self-center bg-slate-200`} />

              <TabList items={versions}>
                {(item) => (
                  <Tab
                    id={pipe(item, Schema.encodeSync(HistoryTabId), (_) => JSON.stringify(_))}
                    className={({ isSelected }) =>
                      twJoin(
                        tw`flex cursor-pointer items-center gap-1.5 rounded-md px-3 py-1.5 text-md font-semibold leading-5 text-slate-800`,
                        isSelected && tw`bg-slate-200`,
                      )
                    }
                  >
                    {Ulid.construct(exampleId).time.toLocaleString()}
                  </Tab>
                )}
              </TabList>
            </div>
          </div>

          <div className={tw`flex h-full min-w-0 flex-1 flex-col`}>
            <Collection items={versions}>
              {(item) => (
                <TabPanel
                  id={pipe(item, Schema.encodeSync(HistoryTabId), (_) => JSON.stringify(_))}
                  className={tw`h-full`}
                >
                  <Suspense
                    fallback={
                      <div className={tw`flex h-full items-center justify-center`}>
                        <Spinner className={tw`size-12`} />
                      </div>
                    }
                  >
                    <ExampleVersionsView item={item} />
                  </Suspense>
                </TabPanel>
              )}
            </Collection>
          </div>
        </Tabs>
      </Dialog>
    </Modal>
  );
};

interface ExampleVersionsViewProps {
  item: ExampleVersionsItem;
}

const ExampleVersionsView = ({ item: { endpointId, exampleId, lastResponseId } }: ExampleVersionsViewProps) => {
  const { data: endpoint } = useConnectSuspenseQuery(endpointGet, { endpointId });

  const url = useEndpointUrl({ endpointId, exampleId });

  return (
    <PanelGroup direction='vertical'>
      <Panel className={tw`flex flex-col`}>
        <div className='m-5 mb-2 flex items-center gap-3 rounded-lg border border-slate-300 px-3 py-2 shadow-sm'>
          <MethodBadge method={endpoint.method} size='lg' />
          <div className={tw`h-7 w-px bg-slate-200`} />
          <div className={tw`truncate font-medium leading-5 tracking-tight text-slate-800`}>{url}</div>
        </div>

        <EndpointRequestView exampleId={exampleId} isReadOnly />
      </Panel>

      {lastResponseId && (
        <>
          <PanelResizeHandle direction='vertical' />
          <Panel>
            <ResponsePanel responseId={lastResponseId} fullWidth />
          </Panel>
        </>
      )}
    </PanelGroup>
  );
};

interface ResponsePanelProps {
  responseId: Uint8Array;
  fullWidth?: boolean;
  className?: string;
}

export const ResponsePanel = ({ responseId, fullWidth = false, className }: ResponsePanelProps) => {
  const { data: response } = useConnectSuspenseQuery(responseGet, { responseId });

  return (
    <Tabs className={twMerge(tw`flex h-full flex-col pb-4`, className)}>
      <div className={twMerge(tw`flex items-center gap-3 border-b border-slate-200 text-md`, fullWidth && tw`px-4`)}>
        <TabList className={tw`flex items-center gap-3`}>
          <Tab
            id='body'
            className={({ isSelected }) =>
              twMerge(
                tw`-mb-px cursor-pointer border-b-2 border-transparent py-2 text-md font-medium leading-5 tracking-tight text-slate-500 transition-colors`,
                isSelected && tw`border-b-violet-700 text-slate-800`,
              )
            }
          >
            Body
          </Tab>

          <Tab
            id='headers'
            className={({ isSelected }) =>
              twMerge(
                tw`-mb-px cursor-pointer border-b-2 border-transparent py-2 text-md font-medium leading-5 tracking-tight text-slate-500 transition-colors`,
                isSelected && tw`border-b-violet-700 text-slate-800`,
              )
            }
          >
            Headers
          </Tab>

          <Tab
            id='assertions'
            className={({ isSelected }) =>
              twMerge(
                tw`-mb-px cursor-pointer border-b-2 border-transparent py-2 text-md font-medium leading-5 tracking-tight text-slate-500 transition-colors`,
                isSelected && tw`border-b-violet-700 text-slate-800`,
              )
            }
          >
            Test Results
          </Tab>
        </TabList>

        <div className={tw`flex-1`} />

        <div className={tw`flex items-center gap-1 text-xs font-medium leading-5 tracking-tight text-slate-800`}>
          <div className={tw`flex gap-1 p-2`}>
            <span>Status:</span>
            <span className={tw`text-green-600`}>{response.status}</span>
          </div>

          <Separator orientation='vertical' className={tw`h-4`} />

          <div className={tw`flex gap-1 p-2`}>
            <span>Time:</span>
            <span className={tw`text-green-600`}>
              {pipe(Number(response.duration), Duration.millis, Duration.format)}
            </span>
          </div>

          <Separator orientation='vertical' className={tw`h-4`} />

          <div className={tw`flex gap-1 p-2`}>
            <span>Size:</span>
            <span>{formatSize(response.size)}</span>
          </div>

          {/* <Separator orientation='vertical' className={tw`h-4`} />

          <Button variant='ghost' className={tw`px-2 text-xs`}>
            <FiSave className={tw`size-4 text-slate-500`} />
            <span>Save as</span>
            <FiChevronDown className={tw`size-4 text-slate-500`} />
          </Button>

          <Separator orientation='vertical' className={tw`h-4`} />

          <Button variant='ghost' className={tw`px-2 text-xs`}>
            <FiX className={tw`size-4 text-slate-500`} />
            <span>Clear</span>
          </Button>

          <Button variant='ghost' className={tw`p-1`}>
            <FiSidebar className={tw`size-4 text-slate-500`} />
          </Button> */}
        </div>
      </div>

      <div className={twJoin(tw`flex-1 overflow-auto pt-4`, fullWidth && tw`px-4`)}>
        <Suspense
          fallback={
            <div className={tw`flex h-full items-center justify-center`}>
              <Spinner className={tw`size-12`} />
            </div>
          }
        >
          <TabPanel id='body' className={twJoin(tw`flex h-full flex-col gap-4`)}>
            <ResponseBodyView bodyBytes={response.body} />
          </TabPanel>

          <TabPanel id='headers'>
            <ResponseHeaderTable responseId={responseId} />
          </TabPanel>

          <TabPanel id='assertions'>
            <ResponseAssertTable responseId={responseId} />
          </TabPanel>
        </Suspense>
      </div>
    </Tabs>
  );
};

const languages = ['text', 'json', 'html', 'xml'] as const;

interface ResponseBodyViewProps {
  bodyBytes: Uint8Array;
}

const ResponseBodyView = ({ bodyBytes }: ResponseBodyViewProps) => {
  const body = new TextDecoder().decode(bodyBytes);

  return (
    <Tabs className='grid flex-1 grid-cols-[auto_1fr] grid-rows-[auto_1fr] items-start gap-4'>
      <TabList className='flex gap-1 self-start rounded-md border border-slate-100 bg-slate-100 p-0.5 text-xs leading-5 tracking-tight'>
        <Tab
          className={({ isSelected }) =>
            twMerge(
              tw`cursor-pointer rounded bg-transparent px-2 py-0.5 text-slate-400 transition-colors`,
              isSelected && tw`bg-white font-medium text-slate-800 shadow`,
            )
          }
          id='pretty'
        >
          Pretty
        </Tab>
        <Tab
          className={({ isSelected }) =>
            twMerge(
              tw`cursor-pointer rounded bg-transparent px-2 py-0.5 text-slate-400 transition-colors`,
              isSelected && tw`bg-white font-medium text-slate-800 shadow`,
            )
          }
          id='raw'
        >
          Raw
        </Tab>
        <Tab
          className={({ isSelected }) =>
            twMerge(
              tw`cursor-pointer rounded bg-transparent px-2 py-0.5 text-slate-400 transition-colors`,
              isSelected && tw`bg-white font-medium text-slate-800 shadow`,
            )
          }
          id='preview'
        >
          Preview
        </Tab>
      </TabList>

      <TabPanel id='pretty' className='contents'>
        <ResponseBodyPrettyView body={body} />
      </TabPanel>

      <TabPanel id='raw' className='col-span-full font-mono'>
        {body}
      </TabPanel>

      <TabPanel id='preview' className='col-span-full self-stretch'>
        <iframe title='Response preview' srcDoc={body} className='size-full' />
      </TabPanel>
    </Tabs>
  );
};

interface ResponseBodyPrettyViewProps {
  body: string;
}

const ResponseBodyPrettyView = ({ body }: ResponseBodyPrettyViewProps) => {
  const [language, setLanguage] = useState<(typeof languages)[number]>('text');

  const { data: prettierBody } = useQuery({
    initialData: '',
    queryKey: ['prettier', language, body],
    queryFn: async () => {
      if (language === 'text') return body;

      const plugins = await pipe(
        Match.value(language),
        Match.when('json', () => [import('prettier/plugins/estree'), import('prettier/plugins/babel')]),
        Match.when('html', () => [import('prettier/plugins/html')]),
        Match.when('xml', () => [import('@prettier/plugin-xml')]),
        Match.exhaustive,
        Array.map((_) => _.then((_) => _.default)),
        (_) => Promise.all(_),
      );

      const parser = pipe(
        Match.value(language),
        Match.when('json', () => 'json-stringify'),
        Match.orElse((_) => _),
      );

      return await prettierFormat(body, {
        parser,
        plugins,
        singleAttributePerLine: true,
        htmlWhitespaceSensitivity: 'ignore',
        xmlWhitespaceSensitivity: 'ignore',
      }).catch(() => body);
    },
  });

  const { data: extensions } = useQuery({
    initialData: [],
    queryKey: ['code-mirror', language],
    queryFn: async () => {
      if (language === 'text') return [];
      return await pipe(
        Match.value(language),
        Match.when('json', () => import('@codemirror/lang-json').then((_) => _.json())),
        Match.when('html', () => import('@codemirror/lang-html').then((_) => _.html())),
        Match.when('xml', () => import('@codemirror/lang-xml').then((_) => _.xml())),
        Match.exhaustive,
        (_) => _.then(Array.make),
      );
    },
  });

  return (
    <>
      <Select
        aria-label='Language'
        className='self-center justify-self-start'
        triggerClassName={tw`px-4 py-1`}
        selectedKey={language}
        onSelectionChange={(_) => void setLanguage(_ as (typeof languages)[number])}
      >
        {languages.map((_) => (
          <ListBoxItem key={_} id={_}>
            {_}
          </ListBoxItem>
        ))}
      </Select>

      <CodeMirror
        value={prettierBody}
        readOnly
        height='100%'
        className='col-span-full self-stretch'
        extensions={extensions}
      />
    </>
  );
};

interface ResponseHeaderTableProps {
  responseId: Uint8Array;
}

const ResponseHeaderTable = ({ responseId }: ResponseHeaderTableProps) => {
  const {
    data: { items },
  } = useConnectSuspenseQuery(responseHeaderList, { responseId });

  const columns = useMemo(() => {
    const { accessor } = createColumnHelper<ResponseHeaderListItem>();
    return [accessor('key', {}), accessor('value', {})];
  }, []);

  const table = useReactTable({
    columns,
    data: items,
    getCoreRowModel: getCoreRowModel(),
  });

  return <DataTable table={table} cellClassName={tw`px-5 py-1.5`} />;
};

interface ResponseAssertTableProps {
  responseId: Uint8Array;
}

const ResponseAssertTable = ({ responseId }: ResponseAssertTableProps) => {
  const {
    data: { items },
  } = useConnectSuspenseQuery(responseAssertList, { responseId });

  return (
    <div className={tw`grid grid-cols-[auto_1fr] items-center gap-2 text-sm`}>
      {items.map(({ assert, result }) => {
        if (!assert) return null;
        const assertIdCan = Ulid.construct(assert.assertId).toCanonical();
        return (
          <Fragment key={assertIdCan}>
            <div
              className={twJoin(
                tw`rounded px-2 py-1 text-center font-light uppercase text-white`,
                result ? tw`bg-green-600` : tw`bg-red-600`,
              )}
            >
              {result ? 'Pass' : 'Fail'}
            </div>

            <span>
              {assert.condition!.comparison!.path.map((_) => JSON.stringify(toJson(ReferenceKeySchema, _))).join(' ')}
            </span>
          </Fragment>
        );
      })}
    </div>
  );
};
