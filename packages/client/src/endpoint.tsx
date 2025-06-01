import { create } from '@bufbuild/protobuf';
import { useTransport } from '@connectrpc/connect-query';
import { useController, useSuspense } from '@data-client/react';
import { makeUrl } from '@effect/platform/UrlParams';
import { effectTsResolver } from '@hookform/resolvers/effect-ts';
import { QueryErrorResetBoundary, useQuery } from '@tanstack/react-query';
import { createFileRoute, getRouteApi, useMatchRoute, useNavigate } from '@tanstack/react-router';
import { createColumnHelper } from '@tanstack/react-table';
import CodeMirror from '@uiw/react-codemirror';
import { Array, Duration, Either, Match, MutableHashMap, Option, pipe, Schema, Struct } from 'effect';
import { Ulid } from 'id128';
import { format as prettierFormat } from 'prettier/standalone';
import { Fragment, Suspense, useMemo, useState } from 'react';
import { Collection, Dialog, DialogTrigger, MenuTrigger, Tab, TabList, TabPanel, Tabs } from 'react-aria-components';
import { useForm } from 'react-hook-form';
import { FiClock, FiMoreHorizontal, FiSave } from 'react-icons/fi';
import { Panel, PanelGroup } from 'react-resizable-panels';
import { twJoin, twMerge } from 'tailwind-merge';

import {
  ExampleBreadcrumbKindSchema,
  ExampleVersionsItem,
} from '@the-dev-tools/spec/collection/item/example/v1/example_pb';
import {
  QueryCreateRequest,
  QueryCreateRequestSchema,
  QueryUpdateRequest,
  QueryUpdateRequestSchema,
} from '@the-dev-tools/spec/collection/item/request/v1/request_pb';
import { ResponseHeaderListItem } from '@the-dev-tools/spec/collection/item/response/v1/response_pb';
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
import { formatSize } from '@the-dev-tools/ui/utils';
import { enumToString } from '~api/utils';
import {
  CodeMirrorMarkupLanguage,
  CodeMirrorMarkupLanguages,
  useCodeMirrorLanguageExtensions,
} from '~code-mirror/extensions';
import { useMutate } from '~data-client';

import { AssertionView } from './assertions';
import { BodyView } from './body';
import { ErrorComponent } from './error';
import { HeaderTable } from './headers';
import { QueryTable } from './query';
import { ReferenceContext } from './reference';
import { StatusBar } from './status-bar';

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
  errorComponent: (props) => (
    <Panel id='main' order={2}>
      <ErrorComponent {...props} />
    </Panel>
  ),
  shouldReload: false,
});

function Page() {
  const { endpointId, exampleId } = Route.useLoaderData();
  const { workspaceId } = workspaceRoute.useLoaderData();

  return (
    <Panel id='main' order={2}>
      <Suspense
        fallback={
          <div className={tw`flex h-full items-center justify-center`}>
            <Spinner className={tw`size-16`} />
          </div>
        }
      >
        <PanelGroup direction='vertical'>
          <Panel className='flex h-full flex-col' id='request' order={1}>
            <EndpointForm endpointId={endpointId} exampleId={exampleId} />

            <ReferenceContext value={{ exampleId, workspaceId }}>
              <EndpointRequestView exampleId={exampleId} />
            </ReferenceContext>
          </Panel>
          <ResponsePanel />
          <StatusBar />
        </PanelGroup>
      </Suspense>
    </Panel>
  );
}

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceIdCan');

interface EndpointRequestViewProps {
  className?: string;
  deltaExampleId?: Uint8Array;
  exampleId: Uint8Array;
  isReadOnly?: boolean;
}

export const EndpointRequestView = ({ className, deltaExampleId, exampleId, isReadOnly }: EndpointRequestViewProps) => (
  <Tabs className={twMerge(tw`flex flex-1 flex-col gap-6 overflow-auto p-6 pt-4`, className)}>
    <TabList className={tw`flex gap-3 border-b border-slate-200`}>
      <Tab
        className={({ isSelected }) =>
          twMerge(
            tw`text-md -mb-px cursor-pointer border-b-2 border-transparent py-1.5 font-medium leading-5 tracking-tight text-slate-500 transition-colors`,
            isSelected && tw`border-b-violet-700 text-slate-800`,
          )
        }
        id='params'
      >
        Params
      </Tab>

      <Tab
        className={({ isSelected }) =>
          twMerge(
            tw`text-md -mb-px cursor-pointer border-b-2 border-transparent py-1.5 font-medium leading-5 tracking-tight text-slate-500 transition-colors`,
            isSelected && tw`border-b-violet-700 text-slate-800`,
          )
        }
        id='headers'
      >
        Headers
      </Tab>

      <Tab
        className={({ isSelected }) =>
          twMerge(
            tw`text-md -mb-px cursor-pointer border-b-2 border-transparent py-1.5 font-medium leading-5 tracking-tight text-slate-500 transition-colors`,
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
            tw`text-md -mb-px cursor-pointer border-b-2 border-transparent py-1.5 font-medium leading-5 tracking-tight text-slate-500 transition-colors`,
            isSelected && tw`border-b-violet-700 text-slate-800`,
          )
        }
        id='assertions'
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

const methods = ['GET', 'HEAD', 'POST', 'PUT', 'DELETE', 'CONNECT', 'OPTION', 'TRACE', 'PATCH'] as const;

interface UseEndpointUrlProps {
  endpointId: Uint8Array;
  exampleId: Uint8Array;
}

export const useEndpointUrl = ({ endpointId, exampleId }: UseEndpointUrlProps) => {
  const transport = useTransport();

  // TODO: fetch in parallel
  const endpoint = useSuspense(EndpointGetEndpoint, transport, { endpointId });
  const { items: queries } = useSuspense(QueryListEndpoint, transport, { exampleId });

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
  const transport = useTransport();
  const controller = useController();

  const matchRoute = useMatchRoute();
  const navigate = useNavigate();

  // TODO: fetch in parallel
  const endpoint = useSuspense(EndpointGetEndpoint, transport, { endpointId });
  const example = useSuspense(ExampleGetEndpoint, transport, { exampleId });
  const { items: queries } = useSuspense(QueryListEndpoint, transport, { exampleId });

  const [exampleUpdate, exampleUpdateLoading] = useMutate(ExampleUpdateEndpoint);

  const url = useEndpointUrl({ endpointId, exampleId });

  const values = useMemo(() => {
    return new EndpointFormData({
      method: Array.contains(methods, endpoint.method) ? endpoint.method : 'N/A',
      url,
    });
  }, [endpoint.method, url]);

  const form = useForm({
    resolver: effectTsResolver(EndpointFormData),
    values,
  });

  const onSubmit = form.handleSubmit(async ({ method, url: urlString }) => {
    const { origin = '', pathname = '', searchParams = new URLSearchParams() } = !urlString ? {} : new URL(urlString);

    await controller.fetch(EndpointUpdateEndpoint, transport, { endpointId, method, url: origin + pathname });

    const queryMap = pipe(
      searchParams.entries(),
      Array.fromIterable,
      Array.map(([key, value]): [string, QueryCreateRequest | QueryUpdateRequest] => [
        key + value,
        create(QueryCreateRequestSchema, { exampleId, key, value }),
      ]),
      MutableHashMap.fromIterable,
    );

    queries.forEach(({ enabled, key, queryId, value }) => {
      MutableHashMap.modifyAt(
        queryMap,
        key + value,
        Option.match({
          onNone: () => {
            if (!enabled) return Option.none();
            return Option.some(create(QueryUpdateRequestSchema, { enabled: false, queryId }));
          },
          onSome: () => {
            if (enabled) return Option.none();
            return Option.some(create(QueryUpdateRequestSchema, { enabled: true, queryId }));
          },
        }),
      );
    });

    await pipe(
      Array.fromIterable(queryMap),
      Array.map(async ([_, query]) => {
        if (query.$typeName === 'collection.item.request.v1.QueryUpdateRequest') {
          await controller.fetch(QueryUpdateEndpoint, transport, query);
        } else {
          await controller.fetch(QueryCreateEndpoint, transport, query);
        }
      }),
      (_) => Promise.allSettled(_),
    );
  });

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => exampleUpdate(transport, { exampleId, name: _ }),
    value: example.name,
  });

  return (
    <form onSubmit={onSubmit}>
      <div className='flex items-center gap-2 border-b border-slate-200 px-4 py-2.5'>
        <div
          className={tw`text-md flex min-w-0 flex-1 select-none gap-1 font-medium leading-5 tracking-tight text-slate-400`}
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
            <h2 className={tw`max-w-full cursor-pointer truncate text-slate-800`} onContextMenu={onContextMenu}>
              {example.name}
            </h2>
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

        <Button className={tw`px-2 py-1 text-slate-800`} type='submit' variant='ghost'>
          <FiSave className={tw`size-4 text-slate-500`} /> Save
        </Button>

        <MenuTrigger {...menuTriggerProps}>
          <Button className={tw`p-1`} variant='ghost'>
            <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
          </Button>

          <Menu {...menuProps}>
            <MenuItem
              onAction={() => controller.fetch(ExampleCreateEndpoint, transport, { endpointId, name: 'New Example' })}
            >
              Add example
            </MenuItem>

            <Separator />

            <MenuItem onAction={() => void edit()}>Rename</MenuItem>

            <MenuItem
              onAction={async () => {
                await controller.fetch(ExampleDeleteEndpoint, transport, { exampleId });
                if (
                  !matchRoute({
                    params: { endpointIdCan: Ulid.construct(endpointId).toCanonical() },
                    to: '/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan',
                  })
                )
                  return;
                await navigate({ from: Route.fullPath, to: '/workspace/$workspaceIdCan' });
              }}
              variant='danger'
            >
              Delete
            </MenuItem>
          </Menu>
        </MenuTrigger>
      </div>

      <div className={tw`flex gap-3 p-6 pb-0`}>
        <div className='shadow-xs flex flex-1 items-center gap-3 rounded-lg border border-slate-300 px-3 py-2'>
          <SelectRHF aria-label='Method' control={form.control} name='method' triggerClassName={tw`border-none p-0`}>
            {methods.map((_) => (
              <ListBoxItem id={_} key={_} textValue={_}>
                <MethodBadge method={_} size='lg' />
              </ListBoxItem>
            ))}
          </SelectRHF>

          <Separator className={tw`h-7`} orientation='vertical' />

          <TextFieldRHF
            aria-label='URL'
            className={tw`flex-1`}
            control={form.control}
            inputClassName={tw`border-none font-medium tracking-tight`}
            name='url'
            onBlur={onSubmit}
          />
        </div>

        <Button
          className={tw`px-6`}
          onPress={async () => {
            await onSubmit();
            await controller.fetch(ExampleRunEndpoint, transport, { exampleId });
          }}
          variant='primary'
        >
          Send
        </Button>
      </div>
    </form>
  );
};

interface HistoryModalProps {
  endpointId: Uint8Array;
  exampleId: Uint8Array;
}

const HistoryModal = ({ endpointId, exampleId }: HistoryModalProps) => {
  const transport = useTransport();

  const { items: versions } = useSuspense(ExampleVersionsEndpoint, transport, { exampleId });

  return (
    <Modal isDismissable modalSize='lg'>
      <Dialog className={tw`outline-hidden size-full`}>
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

              <div className={tw`text-md p-2 font-semibold leading-5 tracking-tight text-violet-700`}>
                Current Version
              </div>

              <div className={tw`flex flex-col items-center gap-0.5`}>
                <div className={tw`w-px flex-1 bg-slate-200`} />
                <div className={tw`size-2 rounded-full bg-slate-300`} />
                <div className={tw`w-px flex-1 bg-slate-200`} />
              </div>

              <div className={tw`text-md p-2 font-semibold leading-5 tracking-tight text-slate-800`}>
                {versions.length} previous responses
              </div>

              <div className={tw`mb-2 w-px flex-1 justify-self-center bg-slate-200`} />

              <TabList items={versions}>
                {(item) => (
                  <Tab
                    className={({ isSelected }) =>
                      twJoin(
                        tw`text-md flex cursor-pointer items-center gap-1.5 rounded-md px-3 py-1.5 font-semibold leading-5 text-slate-800`,
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
                        <Spinner className={tw`size-12`} />
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
  const transport = useTransport();

  const endpoint = useSuspense(EndpointGetEndpoint, transport, { endpointId });

  const url = useEndpointUrl({ endpointId, exampleId });

  return (
    <PanelGroup direction='vertical'>
      <Panel className={tw`flex flex-col`}>
        <div className='shadow-xs m-5 mb-2 flex items-center gap-3 rounded-lg border border-slate-300 px-3 py-2'>
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
            <ResponseTabs fullWidth responseId={lastResponseId} />
          </Panel>
        </>
      )}
    </PanelGroup>
  );
};

const ResponsePanel = () => {
  const transport = useTransport();
  const { exampleId } = Route.useLoaderData();

  const { lastResponseId } = useSuspense(ExampleGetEndpoint, transport, { exampleId });

  if (!lastResponseId) return null;

  return (
    <>
      <PanelResizeHandle direction='vertical' />
      <Panel defaultSize={40} id='response' order={2}>
        <ResponseTabs fullWidth responseId={lastResponseId} />
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
  const transport = useTransport();

  const response = useSuspense(ResponseGetEndpoint, transport, { responseId });

  return (
    <Tabs className={twMerge(tw`flex h-full flex-col pb-4`, className)}>
      <div className={twMerge(tw`text-md flex items-center gap-3 border-b border-slate-200`, fullWidth && tw`px-4`)}>
        <TabList className={tw`flex items-center gap-3`}>
          <Tab
            className={({ isSelected }) =>
              twMerge(
                tw`text-md -mb-px cursor-pointer border-b-2 border-transparent py-2 font-medium leading-5 tracking-tight text-slate-500 transition-colors`,
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
                tw`text-md -mb-px cursor-pointer border-b-2 border-transparent py-2 font-medium leading-5 tracking-tight text-slate-500 transition-colors`,
                isSelected && tw`border-b-violet-700 text-slate-800`,
              )
            }
            id='headers'
          >
            Headers
          </Tab>

          <Tab
            className={({ isSelected }) =>
              twMerge(
                tw`text-md -mb-px cursor-pointer border-b-2 border-transparent py-2 font-medium leading-5 tracking-tight text-slate-500 transition-colors`,
                isSelected && tw`border-b-violet-700 text-slate-800`,
              )
            }
            id='assertions'
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

          <Separator className={tw`h-4`} orientation='vertical' />

          <div className={tw`flex gap-1 p-2`}>
            <span>Time:</span>
            <span className={tw`text-green-600`}>
              {pipe(Number(response.duration), Duration.millis, Duration.format)}
            </span>
          </div>

          <Separator className={tw`h-4`} orientation='vertical' />

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
          <TabPanel className={twJoin(tw`flex h-full flex-col gap-4`)} id='body'>
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

  const { data: prettierBody } = useQuery({
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
  const transport = useTransport();

  const { items } = useSuspense(ResponseHeaderListEndpoint, transport, { responseId });

  const columns = useMemo(() => {
    const { accessor } = createColumnHelper<ResponseHeaderListItem>();
    return [accessor('key', {}), accessor('value', {})];
  }, []);

  const table = useReactTable({
    columns,
    data: items,
  });

  return <DataTable cellClassName={tw`px-5 py-1.5`} table={table} />;
};

interface ResponseAssertTableProps {
  responseId: Uint8Array;
}

const ResponseAssertTable = ({ responseId }: ResponseAssertTableProps) => {
  const transport = useTransport();

  const { items } = useSuspense(ResponseAssertListEndpoint, transport, { responseId });

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
                tw`rounded-sm px-2 py-1 text-center font-light uppercase text-white`,
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
