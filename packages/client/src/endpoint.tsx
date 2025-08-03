import { QueryErrorResetBoundary, useQuery as useReactQuery } from '@tanstack/react-query';
import { createFileRoute, getRouteApi, useNavigate, useRouteContext } from '@tanstack/react-router';
import { createColumnHelper } from '@tanstack/react-table';
import CodeMirror from '@uiw/react-codemirror';
import { Array, Duration, Match, MutableHashMap, Option, pipe, Schema, String, Struct } from 'effect';
import { Ulid } from 'id128';
import { format as prettierFormat } from 'prettier/standalone';
import { Fragment, Suspense, useMemo, useState } from 'react';
import {
  Button as AriaButton,
  Collection,
  Dialog,
  DialogTrigger,
  MenuTrigger,
  Tab,
  TabList,
  TabPanel,
  Tabs,
} from 'react-aria-components';
import { useForm } from 'react-hook-form';
import { FiClock, FiMoreHorizontal } from 'react-icons/fi';
import { Panel, PanelGroup } from 'react-resizable-panels';
import { twJoin, twMerge } from 'tailwind-merge';
import {
  ExampleBreadcrumbKindSchema,
  ExampleVersionsItem,
} from '@the-dev-tools/spec/collection/item/example/v1/example_pb';
import { QueryDeltaListItem } from '@the-dev-tools/spec/collection/item/request/v1/request_pb';
import { ResponseHeaderListItem } from '@the-dev-tools/spec/collection/item/response/v1/response_pb';
import { SourceKind } from '@the-dev-tools/spec/delta/v1/delta_pb';
import {
  EndpointGetEndpoint,
  EndpointUpdateEndpoint,
} from '@the-dev-tools/spec/meta/collection/item/endpoint/v1/endpoint.endpoints.ts';
import {
  ExampleCreateEndpoint,
  ExampleDeleteEndpoint,
  ExampleGetEndpoint,
  ExampleRunEndpoint,
  ExampleUpdateEndpoint,
  ExampleVersionsEndpoint,
} from '@the-dev-tools/spec/meta/collection/item/example/v1/example.endpoints.ts';
import {
  QueryCreateEndpoint,
  QueryDeltaCreateEndpoint,
  QueryDeltaListEndpoint,
  QueryDeltaUpdateEndpoint,
  QueryListEndpoint,
  QueryUpdateEndpoint,
} from '@the-dev-tools/spec/meta/collection/item/request/v1/request.endpoints.ts';
import {
  ResponseAssertListEndpoint,
  ResponseGetEndpoint,
  ResponseHeaderListEndpoint,
} from '@the-dev-tools/spec/meta/collection/item/response/v1/response.endpoints.ts';
import { Button } from '@the-dev-tools/ui/button';
import { DataTable, useReactTable } from '@the-dev-tools/ui/data-table';
import { ListBoxItem } from '@the-dev-tools/ui/list-box';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { MethodBadge } from '@the-dev-tools/ui/method-badge';
import { Modal } from '@the-dev-tools/ui/modal';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { addTab, useRemoveTab } from '@the-dev-tools/ui/router';
import { Select } from '@the-dev-tools/ui/select';
import { Separator } from '@the-dev-tools/ui/separator';
import { Spinner } from '@the-dev-tools/ui/spinner';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { formatSize } from '@the-dev-tools/ui/utils';
import { enumToString, GenericMessage } from '~api/utils';
import {
  CodeMirrorMarkupLanguage,
  CodeMirrorMarkupLanguages,
  useCodeMirrorLanguageExtensions,
} from '~code-mirror/extensions';
import { useMutate, useQuery } from '~data-client';
import { AssertionView } from './assertions';
import { BodyView } from './body';
import { ErrorComponent } from './error';
import { HeaderTable } from './headers';
import { QueryTable } from './query';
import { ReferenceContext, ReferenceFieldRHF } from './reference';

export class EndpointRouteSearch extends Schema.Class<EndpointRouteSearch>('EndpointRouteSearch')({
  responseIdCan: pipe(Schema.String, Schema.optional),
}) {}

const makeRoute = createFileRoute(
  '/_authorized/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan',
);

export const Route = makeRoute({
  validateSearch: (_) => Schema.decodeSync(EndpointRouteSearch)(_),
  loaderDeps: (_) => Struct.pick(_.search, 'responseIdCan'),
  loader: ({ deps: { responseIdCan }, params: { endpointIdCan, exampleIdCan } }) => {
    const endpointId = Ulid.fromCanonical(endpointIdCan).bytes;
    const exampleId = Ulid.fromCanonical(exampleIdCan).bytes;
    const responseId = pipe(
      Option.fromNullable(responseIdCan),
      Option.map((_) => Ulid.fromCanonical(_).bytes),
    );

    return { endpointId, exampleId, responseId };
  },
  component: () => (
    <QueryErrorResetBoundary>
      <Page />
    </QueryErrorResetBoundary>
  ),
  errorComponent: (props) => <ErrorComponent {...props} />,
  onEnter: (match) => {
    if (!match.loaderData) return;

    const { endpointId, exampleId } = match.loaderData;

    addTab({
      id: endpointTabId({ endpointId, exampleId }),
      match,
      node: <EndpointTab endpointId={endpointId} />,
    });
  },
  shouldReload: false,
});

interface EndpointTabIdProps {
  endpointId: Uint8Array;
  exampleId: Uint8Array;
}

export const endpointTabId = ({ endpointId, exampleId }: EndpointTabIdProps) =>
  JSON.stringify({ endpointId, exampleId, route: Route.id });

export const useOnEndpointDelete = () => {
  const context = workspaceRoute.useRouteContext();
  const removeTab = useRemoveTab();
  return (props: EndpointTabIdProps) => removeTab({ ...context, id: endpointTabId(props) });
};

function Page() {
  const { endpointId, exampleId } = Route.useLoaderData();
  const { workspaceId } = workspaceRoute.useLoaderData();

  return (
    <Suspense
      fallback={
        <div className={tw`flex h-full items-center justify-center`}>
          <Spinner size='xl' />
        </div>
      }
    >
      <PanelGroup direction='vertical'>
        <Panel className='flex h-full flex-col' id='request' order={1}>
          <ReferenceContext value={{ exampleId, workspaceId }}>
            <EndpointHeader endpointId={endpointId} exampleId={exampleId} />

            <EndpointRequestView exampleId={exampleId} />
          </ReferenceContext>
        </Panel>
        <Suspense>
          <ResponsePanel />
        </Suspense>
      </PanelGroup>
    </Suspense>
  );
}

interface EndpointTabProps {
  endpointId: Uint8Array;
}

const EndpointTab = ({ endpointId }: EndpointTabProps) => {
  const { method, name } = useQuery(EndpointGetEndpoint, { endpointId });
  return (
    <>
      {method && <MethodBadge method={method} />}
      <span className={tw`min-w-0 flex-1 truncate`}>{name}</span>
    </>
  );
};

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceIdCan');

interface EndpointRequestViewProps {
  className?: string;
  deltaExampleId?: Uint8Array;
  exampleId: Uint8Array;
  isReadOnly?: boolean;
}

export const EndpointRequestView = ({ className, deltaExampleId, exampleId, isReadOnly }: EndpointRequestViewProps) => {
  const { assertCount, bodyCount, headerCount, queryCount } = useQuery(ExampleGetEndpoint, { exampleId });

  return (
    <Tabs className={twMerge(tw`flex flex-1 flex-col gap-6 overflow-auto p-6 pt-4`, className)}>
      <TabList className={tw`flex gap-3 border-b border-slate-200`}>
        <Tab
          className={({ isSelected }) =>
            twMerge(
              tw`
                -mb-px cursor-pointer border-b-2 border-transparent py-1.5 text-md leading-5 font-medium tracking-tight
                text-slate-500 transition-colors
              `,
              isSelected && tw`border-b-violet-700 text-slate-800`,
            )
          }
          id='params'
        >
          Params
          {queryCount > 0 && <span className={tw`text-xs text-green-600`}> ({queryCount})</span>}
        </Tab>

        <Tab
          className={({ isSelected }) =>
            twMerge(
              tw`
                -mb-px cursor-pointer border-b-2 border-transparent py-1.5 text-md leading-5 font-medium tracking-tight
                text-slate-500 transition-colors
              `,
              isSelected && tw`border-b-violet-700 text-slate-800`,
            )
          }
          id='headers'
        >
          Headers
          {headerCount > 0 && <span className={tw`text-xs text-green-600`}> ({headerCount})</span>}
        </Tab>

        <Tab
          className={({ isSelected }) =>
            twMerge(
              tw`
                -mb-px cursor-pointer border-b-2 border-transparent py-1.5 text-md leading-5 font-medium tracking-tight
                text-slate-500 transition-colors
              `,
              isSelected && tw`border-b-violet-700 text-slate-800`,
            )
          }
          id='body'
        >
          Body
          {bodyCount > 0 && <span className={tw`text-xs text-green-600`}> ({bodyCount})</span>}
        </Tab>

        <Tab
          className={({ isSelected }) =>
            twMerge(
              tw`
                -mb-px cursor-pointer border-b-2 border-transparent py-1.5 text-md leading-5 font-medium tracking-tight
                text-slate-500 transition-colors
              `,
              isSelected && tw`border-b-violet-700 text-slate-800`,
            )
          }
          id='assertions'
        >
          Assertion
          {assertCount > 0 && <span className={tw`text-xs text-green-600`}> ({assertCount})</span>}
        </Tab>
      </TabList>

      <Suspense
        fallback={
          <div className={tw`flex h-full items-center justify-center`}>
            <Spinner size='lg' />
          </div>
        }
      >
        <TabPanel id='params'>
          <QueryTable deltaExampleId={deltaExampleId} exampleId={exampleId} isReadOnly={isReadOnly} />
        </TabPanel>

        <TabPanel id='headers'>
          <HeaderTable deltaExampleId={deltaExampleId} exampleId={exampleId} isReadOnly={isReadOnly} />
        </TabPanel>

        <TabPanel id='body'>
          <BodyView deltaExampleId={deltaExampleId} exampleId={exampleId} isReadOnly={isReadOnly} />
        </TabPanel>

        <TabPanel id='assertions'>
          <AssertionView exampleId={exampleId} isReadOnly={isReadOnly} />
        </TabPanel>
      </Suspense>
    </Tabs>
  );
};

const queryToString = (query: GenericMessage<QueryDeltaListItem>): Option.Option<string> => {
  if (query.source === SourceKind.ORIGIN && query.origin) return queryToString(query.origin);

  return pipe(
    query,
    Option.liftPredicate((_) => _.enabled),
    Option.map((_) => _.key + '=' + _.value),
  );
};

const queryFromString = (query: string) =>
  pipe(
    query,
    String.indexOf('='),
    Option.match({
      onNone: () => [query, ''] as const,
      onSome: (pos) => [query.slice(0, pos), query.slice(pos + 1)] as const,
    }),
  );

const methods = ['GET', 'HEAD', 'POST', 'PUT', 'DELETE', 'CONNECT', 'OPTION', 'TRACE', 'PATCH'] as const;

interface UseEndpointUrlProps {
  deltaEndpointId?: Uint8Array | undefined;
  deltaExampleId?: Uint8Array | undefined;
  endpointId: Uint8Array;
  exampleId: Uint8Array;
}

export const useEndpointUrl = ({ deltaEndpointId, deltaExampleId, endpointId, exampleId }: UseEndpointUrlProps) => {
  // TODO: fetch in parallel
  const endpoint = useQuery(EndpointGetEndpoint, { endpointId });
  const queryList = useQuery(QueryListEndpoint, { exampleId });

  const deltaEndpoint = useQuery(EndpointGetEndpoint, deltaEndpointId ? { endpointId: deltaEndpointId } : null);

  const deltaQueryList = useQuery(
    QueryDeltaListEndpoint,
    deltaExampleId ? { exampleId: deltaExampleId, originId: exampleId } : null,
  );

  // eslint-disable-next-line @typescript-eslint/prefer-nullish-coalescing
  let url = deltaEndpoint?.url || endpoint.url;
  const queries = deltaQueryList.items ?? queryList.items;

  const queryParams = pipe(queries, Array.filterMap(queryToString), Array.join('&'));

  if (queryParams.length > 0) url = url + '?' + queryParams;

  return url;
};

interface UseEndpointUrlFormProps {
  deltaEndpointId?: Uint8Array;
  deltaExampleId?: Uint8Array;
  endpointId: Uint8Array;
  exampleId: Uint8Array;
}

export const useEndpointUrlForm = ({
  deltaEndpointId,
  deltaExampleId,
  endpointId,
  exampleId,
}: UseEndpointUrlFormProps) => {
  const { dataClient } = useRouteContext({ from: '__root__' });

  // TODO: fetch in parallel
  const endpoint = useQuery(EndpointGetEndpoint, { endpointId });
  const queryList = useQuery(QueryListEndpoint, { exampleId });

  const deltaEndpoint = useQuery(EndpointGetEndpoint, deltaEndpointId ? { endpointId: deltaEndpointId } : null);

  const deltaQueryList = useQuery(
    QueryDeltaListEndpoint,
    deltaExampleId ? { exampleId: deltaExampleId, originId: exampleId } : null,
  );

  const method = deltaEndpoint?.method ?? endpoint.method;
  const queries: GenericMessage<QueryDeltaListItem>[] = deltaQueryList.items ?? queryList.items;

  const url = useEndpointUrl({ deltaEndpointId, deltaExampleId, endpointId, exampleId });

  const form = useForm({ values: { url } });

  const submit = form.handleSubmit(async ({ url: urlString }) => {
    const { queryString, url } = pipe(
      urlString,
      String.indexOf('?'),
      Option.match({
        onNone: () => ({ queryString: '', url: urlString }),
        onSome: (pos) => ({ queryString: urlString.slice(pos + 1), url: urlString.slice(0, pos) }),
      }),
    );

    if (deltaEndpointId) {
      await dataClient.fetch(EndpointUpdateEndpoint, { endpointId: deltaEndpointId, url });
    } else {
      await dataClient.fetch(EndpointUpdateEndpoint, { endpointId, url });
    }

    type Change = { key: string; value: string } & (
      | { enabled: boolean; isNew: false; queryId: Uint8Array; source: SourceKind | undefined }
      | { isNew: true }
    );

    const changeMapMap = pipe(
      queryString.length > 0 ? queryString.split('&') : [],
      Array.map(queryFromString),
      Array.map(([key, value]): [string, Change] => [key + value, { isNew: true, key, value }]),
      MutableHashMap.fromIterable,
    );

    queries.forEach(({ origin, queryId, source, ...data }) => {
      let { enabled, key, value } = data;
      if (source === SourceKind.ORIGIN && origin) ({ enabled, key, value } = origin);

      MutableHashMap.modifyAt(
        changeMapMap,
        key + value,
        Option.match({
          onNone: () => {
            if (!enabled) return Option.none();
            return Option.some<Change>({ enabled: false, isNew: false, key, queryId, source, value });
          },
          onSome: () => {
            if (enabled) return Option.none();
            return Option.some<Change>({ enabled: true, isNew: false, key, queryId, source, value });
          },
        }),
      );
    });

    await pipe(
      Array.fromIterable(changeMapMap),
      Array.map(async ([_, change]) => {
        if (change.isNew) {
          const { key, value } = change;
          if (deltaExampleId) {
            await dataClient.fetch(QueryDeltaCreateEndpoint, {
              enabled: true,
              exampleId: deltaExampleId,
              key,
              originId: exampleId,
              value,
            });
          } else {
            await dataClient.fetch(QueryCreateEndpoint, { enabled: true, exampleId, key, value });
          }
        } else {
          const { enabled, key, queryId, source, value } = change;
          if (deltaExampleId) {
            await dataClient.fetch(QueryDeltaUpdateEndpoint, {
              enabled,
              key,
              queryId,
              source: source!,
              value,
            });
          } else {
            await dataClient.fetch(QueryUpdateEndpoint, { enabled, queryId });
          }
        }
      }),
      (_) => Promise.allSettled(_),
    );
  });

  const render = (
    <div className='shadow-xs flex flex-1 items-center gap-3 rounded-lg border border-slate-300 px-3 py-2'>
      <Select
        aria-label='Method'
        onSelectionChange={async (method) => {
          if (typeof method !== 'string') return;

          if (deltaEndpointId) {
            await dataClient.fetch(EndpointUpdateEndpoint, { endpointId: deltaEndpointId, method });
          } else {
            await dataClient.fetch(EndpointUpdateEndpoint, { endpointId, method });
          }
        }}
        selectedKey={method}
        triggerClassName={tw`border-none p-0`}
      >
        {methods.map((_) => (
          <ListBoxItem id={_} key={_} textValue={_}>
            <MethodBadge method={_} size='lg' />
          </ListBoxItem>
        ))}
      </Select>

      <Separator className={tw`h-7`} orientation='vertical' />

      <ReferenceFieldRHF
        aria-label='URL'
        className={tw`flex-1 border-none font-medium tracking-tight`}
        control={form.control}
        kind='StringExpression'
        name='url'
        onBlur={submit}
      />
    </div>
  );

  return [render, submit] as const;
};

interface EndpointHeaderProps {
  endpointId: Uint8Array;
  exampleId: Uint8Array;
}

export const EndpointHeader = ({ endpointId, exampleId }: EndpointHeaderProps) => {
  const { dataClient } = useRouteContext({ from: '__root__' });

  const navigate = useNavigate();

  const onEndpointDelete = useOnEndpointDelete();

  const example = useQuery(ExampleGetEndpoint, { exampleId });

  const [exampleUpdate, exampleUpdateLoading] = useMutate(ExampleUpdateEndpoint);

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => exampleUpdate({ exampleId, name: _ }),
    value: example.name,
  });

  const [renderEndpointUrlForm, submitEndpointUrlForm] = useEndpointUrlForm({ endpointId, exampleId });

  const [isSending, setIsSending] = useState(false);

  return (
    <>
      <div className='flex items-center gap-2 border-b border-slate-200 px-4 py-2.5'>
        <div
          className={tw`
            flex min-w-0 flex-1 gap-1 text-md leading-5 font-medium tracking-tight text-slate-400 select-none
          `}
        >
          {example.breadcrumbs.map((_, index) => {
            // TODO: add links to breadcrumbs
            const key = enumToString(ExampleBreadcrumbKindSchema, 'EXAMPLE_BREADCRUMB_KIND', _.kind);
            const name = _[key]?.name;
            return (
              <Fragment key={`${index} ${name}`}>
                <span>{name}</span>
                <span>/</span>
              </Fragment>
            );
          })}

          {isEditing ? (
            <TextField
              aria-label='Example name'
              inputClassName={tw`-my-1 py-1 leading-none text-slate-800`}
              isDisabled={exampleUpdateLoading}
              {...textFieldProps}
            />
          ) : (
            <AriaButton
              className={tw`max-w-full cursor-text truncate text-slate-800`}
              onContextMenu={onContextMenu}
              onPress={() => void edit()}
            >
              {example.name}
            </AriaButton>
          )}
        </div>

        <DialogTrigger>
          <Button className={tw`px-2 py-1 text-slate-800`} variant='ghost'>
            <FiClock className={tw`size-4 text-slate-500`} /> Response History
          </Button>

          <HistoryModal endpointId={endpointId} exampleId={exampleId} />
        </DialogTrigger>

        {/* TODO: implement copy link */}
        {/* <Button variant='ghost' className={tw`px-2 py-1 text-slate-800`}>
          <FiLink className={tw`size-4 text-slate-500`} /> Copy Link
        </Button> */}

        {/* <Separator orientation='vertical' className={tw`h-4`} /> */}

        <MenuTrigger {...menuTriggerProps}>
          <Button className={tw`p-1`} variant='ghost'>
            <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
          </Button>

          <Menu {...menuProps}>
            <MenuItem
              onAction={async () => {
                const { exampleId } = await dataClient.fetch(ExampleCreateEndpoint, {
                  endpointId,
                  name: 'New Example',
                });

                const endpointIdCan = Ulid.construct(endpointId).toCanonical();
                const exampleIdCan = Ulid.construct(exampleId).toCanonical();

                await navigate({
                  from: Route.fullPath,
                  to: Route.fullPath,

                  params: { endpointIdCan, exampleIdCan },
                });
              }}
            >
              Add example
            </MenuItem>

            <Separator />

            <MenuItem onAction={() => void edit()}>Rename</MenuItem>

            <MenuItem
              onAction={async () => {
                await onEndpointDelete({ endpointId, exampleId });
                await dataClient.fetch(ExampleDeleteEndpoint, { exampleId });
              }}
              variant='danger'
            >
              Delete
            </MenuItem>
          </Menu>
        </MenuTrigger>
      </div>

      <div className={tw`flex gap-3 p-6 pb-0`}>
        {renderEndpointUrlForm}

        <Button
          className={tw`px-6`}
          isPending={isSending}
          onPress={async () => {
            try {
              setIsSending(true);
              await submitEndpointUrlForm();
              await dataClient.fetch(ExampleRunEndpoint, { exampleId });
            } finally {
              setIsSending(false);
            }
          }}
          variant='primary'
        >
          Send
        </Button>
      </div>
    </>
  );
};

interface HistoryModalProps {
  endpointId: Uint8Array;
  exampleId: Uint8Array;
}

const HistoryModal = ({ endpointId, exampleId }: HistoryModalProps) => {
  const { items: versions } = useQuery(ExampleVersionsEndpoint, { exampleId });

  return (
    <Modal isDismissable modalSize='lg'>
      <Dialog className={tw`size-full outline-hidden`}>
        <Tabs className={tw`flex h-full`} orientation='vertical'>
          <div className={tw`flex w-64 flex-col border-r border-slate-200 bg-slate-50 p-4 tracking-tight`}>
            <div className={tw`mb-4`}>
              <div className={tw`mb-0.5 text-sm leading-5 font-semibold text-slate-800`}>Response History</div>
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

              <div className={tw`p-2 text-md leading-5 font-semibold tracking-tight text-violet-700`}>
                Current Version
              </div>

              <div className={tw`flex flex-col items-center gap-0.5`}>
                <div className={tw`w-px flex-1 bg-slate-200`} />
                <div className={tw`size-2 rounded-full bg-slate-300`} />
                <div className={tw`w-px flex-1 bg-slate-200`} />
              </div>

              <div className={tw`p-2 text-md leading-5 font-semibold tracking-tight text-slate-800`}>
                {versions.length} previous responses
              </div>

              <div className={tw`mb-2 w-px flex-1 justify-self-center bg-slate-200`} />

              <TabList items={versions}>
                {(item) => (
                  <Tab
                    className={({ isSelected }) =>
                      twJoin(
                        tw`
                          flex cursor-pointer items-center gap-1.5 rounded-md px-3 py-1.5 text-md leading-5
                          font-semibold text-slate-800
                        `,
                        isSelected && tw`bg-slate-200`,
                      )
                    }
                    id={Ulid.construct(item.exampleId).toCanonical()}
                  >
                    {Ulid.construct(item.exampleId).time.toLocaleString()}
                  </Tab>
                )}
              </TabList>
            </div>
          </div>

          <div className={tw`flex h-full min-w-0 flex-1 flex-col`}>
            <Collection items={versions}>
              {(item) => (
                <TabPanel className={tw`h-full`} id={Ulid.construct(item.exampleId).toCanonical()}>
                  <Suspense
                    fallback={
                      <div className={tw`flex h-full items-center justify-center`}>
                        <Spinner size='lg' />
                      </div>
                    }
                  >
                    <ExampleVersionsView endpointId={endpointId} item={item} />
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
  endpointId: Uint8Array;
  item: ExampleVersionsItem;
}

const ExampleVersionsView = ({ endpointId, item: { exampleId, lastResponseId } }: ExampleVersionsViewProps) => {
  const endpoint = useQuery(EndpointGetEndpoint, { endpointId });

  const url = useEndpointUrl({ endpointId, exampleId });

  return (
    <PanelGroup direction='vertical'>
      <Panel className={tw`flex flex-col`}>
        <div className='shadow-xs m-5 mb-2 flex items-center gap-3 rounded-lg border border-slate-300 px-3 py-2'>
          <MethodBadge method={endpoint.method} size='lg' />
          <div className={tw`h-7 w-px bg-slate-200`} />
          <div className={tw`truncate leading-5 font-medium tracking-tight text-slate-800`}>{url}</div>
        </div>

        <EndpointRequestView exampleId={exampleId} isReadOnly />
      </Panel>

      {lastResponseId && (
        <>
          <PanelResizeHandle direction='vertical' />
          <Panel>
            <ResponseTabs fullWidth responseId={lastResponseId} />
          </Panel>
        </>
      )}
    </PanelGroup>
  );
};

const ResponsePanel = () => {
  const { exampleId } = Route.useLoaderData();

  const { lastResponseId } = useQuery(ExampleGetEndpoint, { exampleId });

  if (!lastResponseId) return null;

  return (
    <>
      <PanelResizeHandle direction='vertical' />
      <Panel defaultSize={40} id='response' order={2}>
        <Suspense
          fallback={
            <div className={tw`flex h-full items-center justify-center`}>
              <Spinner size='lg' />
            </div>
          }
        >
          <ResponseTabs fullWidth responseId={lastResponseId} />
        </Suspense>
      </Panel>
    </>
  );
};

interface ResponseTabsProps {
  className?: string;
  fullWidth?: boolean;
  responseId: Uint8Array;
}

export const ResponseTabs = ({ className, fullWidth = false, responseId }: ResponseTabsProps) => {
  const { assertCount, body, duration, headerCount, size, status } = useQuery(ResponseGetEndpoint, {
    responseId,
  });

  return (
    <Tabs className={twMerge(tw`flex h-full flex-col pb-4`, className)}>
      <div className={twMerge(tw`flex items-center gap-3 border-b border-slate-200 text-md`, fullWidth && tw`px-4`)}>
        <TabList className={tw`flex items-center gap-3`}>
          <Tab
            className={({ isSelected }) =>
              twMerge(
                tw`
                  -mb-px cursor-pointer border-b-2 border-transparent py-2 text-md leading-5 font-medium tracking-tight
                  text-slate-500 transition-colors
                `,
                isSelected && tw`border-b-violet-700 text-slate-800`,
              )
            }
            id='body'
          >
            Body
          </Tab>

          <Tab
            className={({ isSelected }) =>
              twMerge(
                tw`
                  -mb-px cursor-pointer border-b-2 border-transparent py-2 text-md leading-5 font-medium tracking-tight
                  text-slate-500 transition-colors
                `,
                isSelected && tw`border-b-violet-700 text-slate-800`,
              )
            }
            id='headers'
          >
            Headers
            {headerCount > 0 && <span className={tw`text-xs text-green-600`}> ({headerCount})</span>}
          </Tab>

          <Tab
            className={({ isSelected }) =>
              twMerge(
                tw`
                  -mb-px cursor-pointer border-b-2 border-transparent py-2 text-md leading-5 font-medium tracking-tight
                  text-slate-500 transition-colors
                `,
                isSelected && tw`border-b-violet-700 text-slate-800`,
              )
            }
            id='assertions'
          >
            Test Results
            {assertCount > 0 && <span className={tw`text-xs text-green-600`}> ({assertCount})</span>}
          </Tab>
        </TabList>

        <div className={tw`flex-1`} />

        <div className={tw`flex items-center gap-1 text-xs leading-5 font-medium tracking-tight text-slate-800`}>
          <div className={tw`flex gap-1 p-2`}>
            <span>Status:</span>
            <span className={tw`text-green-600`}>{status}</span>
          </div>

          <Separator className={tw`h-4`} orientation='vertical' />

          <div className={tw`flex gap-1 p-2`}>
            <span>Time:</span>
            <span className={tw`text-green-600`}>{pipe(Number(duration), Duration.millis, Duration.format)}</span>
          </div>

          <Separator className={tw`h-4`} orientation='vertical' />

          <div className={tw`flex gap-1 p-2`}>
            <span>Size:</span>
            <span>{formatSize(size)}</span>
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
              <Spinner size='lg' />
            </div>
          }
        >
          <TabPanel className={twJoin(tw`flex h-full flex-col gap-4`)} id='body'>
            <ResponseBodyView bodyBytes={body} />
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

interface ResponseBodyViewProps {
  bodyBytes: Uint8Array;
}

const ResponseBodyView = ({ bodyBytes }: ResponseBodyViewProps) => {
  const body = new TextDecoder().decode(bodyBytes);

  return (
    <Tabs
      className='grid flex-1 grid-cols-[auto_1fr] grid-rows-[auto_1fr] items-start gap-4'
      defaultSelectedKey='pretty'
    >
      <TabList className='flex gap-1 self-start rounded-md border border-slate-100 bg-slate-100 p-0.5 text-xs leading-5 tracking-tight'>
        <Tab
          className={({ isSelected }) =>
            twMerge(
              tw`cursor-pointer rounded-sm bg-transparent px-2 py-0.5 text-slate-400 transition-colors`,
              isSelected && tw`bg-white font-medium text-slate-800 shadow-sm`,
            )
          }
          id='pretty'
        >
          Pretty
        </Tab>
        <Tab
          className={({ isSelected }) =>
            twMerge(
              tw`cursor-pointer rounded-sm bg-transparent px-2 py-0.5 text-slate-400 transition-colors`,
              isSelected && tw`bg-white font-medium text-slate-800 shadow-sm`,
            )
          }
          id='raw'
        >
          Raw
        </Tab>
        <Tab
          className={({ isSelected }) =>
            twMerge(
              tw`cursor-pointer rounded-sm bg-transparent px-2 py-0.5 text-slate-400 transition-colors`,
              isSelected && tw`bg-white font-medium text-slate-800 shadow-sm`,
            )
          }
          id='preview'
        >
          Preview
        </Tab>
      </TabList>

      <TabPanel className='contents' id='pretty'>
        <ResponseBodyPrettyView body={body} />
      </TabPanel>

      <TabPanel className='col-span-full overflow-auto whitespace-pre font-mono' id='raw'>
        {body}
      </TabPanel>

      <TabPanel className='col-span-full self-stretch' id='preview'>
        <iframe className='size-full' srcDoc={body} title='Response preview' />
      </TabPanel>
    </Tabs>
  );
};

interface ResponseBodyPrettyViewProps {
  body: string;
}

const ResponseBodyPrettyView = ({ body }: ResponseBodyPrettyViewProps) => {
  const initialLanguage = pipe(
    Match.value(body),
    Match.when(
      (_) => pipe(_, Schema.decodeUnknownOption(Schema.parseJson()), Option.isSome),
      (): CodeMirrorMarkupLanguage => 'json',
    ),
    Match.when(
      (_) => /<\?xml|<[a-z]+:[a-z]+/i.test(_),
      (): CodeMirrorMarkupLanguage => 'xml',
    ),
    Match.when(
      (_) => /<\/?[a-z][\s\S]*>/i.test(_),
      (): CodeMirrorMarkupLanguage => 'html',
    ),
    Match.orElse((): CodeMirrorMarkupLanguage => 'text'),
  );

  const [language, setLanguage] = useState(initialLanguage);

  const { data: prettierBody } = useReactQuery({
    initialData: 'Formatting...',
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
        htmlWhitespaceSensitivity: 'ignore',
        parser,
        plugins,
        printWidth: 100,
        singleAttributePerLine: true,
        tabWidth: 2,
        xmlWhitespaceSensitivity: 'ignore',
      }).catch(() => body);
    },
    queryKey: ['prettier', language, body],
  });

  const extensions = useCodeMirrorLanguageExtensions(language);

  return (
    <>
      <Select
        aria-label='Language'
        className='self-center justify-self-start'
        onSelectionChange={(_) => void setLanguage(_ as CodeMirrorMarkupLanguage)}
        selectedKey={language}
        triggerClassName={tw`px-4 py-1`}
      >
        {CodeMirrorMarkupLanguages.map((_) => (
          <ListBoxItem id={_} key={_}>
            {_}
          </ListBoxItem>
        ))}
      </Select>

      <CodeMirror
        className='col-span-full self-stretch'
        extensions={extensions}
        height='100%'
        indentWithTab={false}
        readOnly
        value={prettierBody}
      />
    </>
  );
};

interface ResponseHeaderTableProps {
  responseId: Uint8Array;
}

const ResponseHeaderTable = ({ responseId }: ResponseHeaderTableProps) => {
  const { items } = useQuery(ResponseHeaderListEndpoint, { responseId });

  const columns = useMemo(() => {
    const { accessor } = createColumnHelper<ResponseHeaderListItem>();
    return [accessor('key', {}), accessor('value', {})];
  }, []);

  const table = useReactTable({
    columns,
    data: items,
  });

  return <DataTable cellClassName={tw`px-5 py-1.5`} table={table} tableAria-label='Response headers' />;
};

interface ResponseAssertTableProps {
  responseId: Uint8Array;
}

const ResponseAssertTable = ({ responseId }: ResponseAssertTableProps) => {
  const { items } = useQuery(ResponseAssertListEndpoint, { responseId });

  return (
    <div className={tw`grid grid-cols-[auto_1fr] items-center gap-2 text-sm`}>
      {items.map(({ assert, result }) => {
        if (!assert) return null;
        const assertIdCan = Ulid.construct(assert.assertId).toCanonical();
        const { expression } = assert.condition!.comparison!;
        return (
          <Fragment key={assertIdCan}>
            <div
              className={twJoin(
                tw`rounded-sm px-2 py-1 text-center font-light text-white uppercase`,
                result ? tw`bg-green-600` : tw`bg-red-600`,
              )}
            >
              {result ? 'Pass' : 'Fail'}
            </div>

            <span>{expression}</span>
          </Fragment>
        );
      })}
    </div>
  );
};
