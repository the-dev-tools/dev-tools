import { useRouteContext } from '@tanstack/react-router';
import { Position, useReactFlow } from '@xyflow/react';
import { Ulid } from 'id128';
import { Suspense, use, useEffect } from 'react';
import { Tooltip, TooltipTrigger } from 'react-aria-components';
import { FiExternalLink, FiX } from 'react-icons/fi';
import { NodeRequest } from '@the-dev-tools/spec/flow/node/v1/node_pb';
import {
  EndpointCreateEndpoint,
  EndpointGetEndpoint,
} from '@the-dev-tools/spec/meta/collection/item/endpoint/v1/endpoint.endpoints.ts';
import {
  ExampleCreateEndpoint,
  ExampleGetEndpoint,
} from '@the-dev-tools/spec/meta/collection/item/example/v1/example.endpoints.ts';
import { CollectionGetEndpoint } from '@the-dev-tools/spec/meta/collection/v1/collection.endpoints.ts';
import { NodeGetEndpoint, NodeUpdateEndpoint } from '@the-dev-tools/spec/meta/flow/node/v1/node.endpoints.js';
import { ButtonAsLink } from '@the-dev-tools/ui/button';
import { SendRequestIcon, Spinner } from '@the-dev-tools/ui/icons';
import { MethodBadge } from '@the-dev-tools/ui/method-badge';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useQuery } from '~data-client';
import { CollectionListTree } from '../../collection';
import { EndpointRequestView, ResponseTabs, useEndpointUrlForm } from '../../endpoint';
import { ReferenceContext } from '../../reference';
import { FlowContext, Handle, workspaceRoute } from '../internal';
import { NodeBody, NodeContainer, NodePanelProps, NodeProps } from '../node';

export const RequestNode = (props: NodeProps) => (
  <NodeContainer {...props} handles={<Handle position={Position.Top} type='target' />}>
    <RequestNodeBody {...props} />
  </NodeContainer>
);

const RequestNodeBody = (props: NodeProps) => {
  const { id, selected } = props;

  const { dataClient } = useRouteContext({ from: '__root__' });
  const { deleteElements } = useReactFlow();

  const nodeId = Ulid.fromCanonical(id).bytes;

  const { request } = useQuery(NodeGetEndpoint, { nodeId });

  useEffect(() => {
    if (!selected && !request?.exampleId.length) void deleteElements({ nodes: [{ id }] });
  }, [deleteElements, id, request?.exampleId.length, selected]);

  return (
    <NodeBody {...props} Icon={SendRequestIcon}>
      <div className={tw`rounded-md border border-slate-200 bg-white shadow-xs`}>
        {request?.exampleId.length !== 0 ? (
          <>
            <RequestNodeSelected request={request!} />
            <Handle position={Position.Bottom} type='source' />
          </>
        ) : (
          <CollectionListTree
            onAction={async ({ collectionId, endpointId, exampleId }) => {
              if (collectionId === undefined || endpointId === undefined || exampleId === undefined) return;

              const {
                endpoint: { endpointId: deltaEndpointId },
              } = await dataClient.fetch(EndpointCreateEndpoint, {
                collectionId,
                hidden: true,
              });

              const { exampleId: deltaExampleId } = await dataClient.fetch(ExampleCreateEndpoint, {
                endpointId: deltaEndpointId,
                hidden: true,
              });

              await dataClient.fetch(NodeUpdateEndpoint, {
                nodeId,
                request: {
                  ...request,
                  collectionId,
                  deltaEndpointId,
                  deltaExampleId,
                  endpointId,
                  exampleId,
                },
              });
            }}
          />
        )}
      </div>
    </NodeBody>
  );
};

interface RequestNodeSelectedProps {
  request: NodeRequest;
}

const RequestNodeSelected = ({
  request: { collectionId, deltaEndpointId, deltaExampleId, endpointId, exampleId },
}: RequestNodeSelectedProps) => {
  // TODO: fetch in parallel
  const { name: collectionName } = useQuery(CollectionGetEndpoint, { collectionId });

  const { workspaceIdCan } = workspaceRoute.useParams();

  const endpoint = useQuery(EndpointGetEndpoint, { endpointId });
  const example = useQuery(ExampleGetEndpoint, { exampleId });

  const deltaEndpoint = useQuery(EndpointGetEndpoint, { endpointId: deltaEndpointId });
  const deltaExample = useQuery(ExampleGetEndpoint, { exampleId: deltaExampleId });

  const method = deltaEndpoint.method || endpoint.method;
  const name = deltaExample.name || deltaEndpoint.name || example.name || endpoint.name;

  return (
    <div className={tw`space-y-1.5 p-2`}>
      <div className={tw`truncate text-xs leading-4 tracking-tight text-slate-400`}>{collectionName}</div>
      <div className={tw`flex items-center gap-1.5`}>
        <MethodBadge method={method} />
        <div className={tw`flex-1 truncate text-xs leading-5 font-medium tracking-tight text-slate-800`}>{name}</div>
        <ButtonAsLink
          className={tw`p-0.5`}
          from='/'
          params={{
            endpointIdCan: Ulid.construct(endpointId).toCanonical(),
            exampleIdCan: Ulid.construct(exampleId).toCanonical(),
            workspaceIdCan,
          }}
          to='/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan'
          variant='ghost'
        >
          <FiExternalLink className={tw`size-4 text-slate-500`} />
        </ButtonAsLink>
      </div>
    </div>
  );
};

export const RequestPanel = ({ node: { nodeId, request } }: NodePanelProps) => {
  const { collectionId, deltaEndpointId, deltaExampleId, endpointId, exampleId } = request!;
  const { isReadOnly = false } = use(FlowContext);

  const { workspaceId } = workspaceRoute.useLoaderData();
  const { workspaceIdCan } = workspaceRoute.useParams();

  // TODO: fetch in parallel
  const collection = useQuery(CollectionGetEndpoint, { collectionId });
  const example = useQuery(ExampleGetEndpoint, { exampleId });

  const { lastResponseId } = example;

  const [renderEndpointUrlForm] = useEndpointUrlForm({
    deltaEndpointId,
    deltaExampleId,
    endpointId,
    exampleId,
  });

  return (
    <>
      <div className={tw`sticky top-0 z-10 flex items-center border-b border-slate-200 bg-white px-5 py-2`}>
        <div className='min-w-0'>
          <div className={tw`text-md leading-5 text-slate-400`}>{collection.name}</div>
          <div className={tw`truncate text-sm leading-5 font-medium text-slate-800`}>{example.name}</div>
        </div>

        <div className={tw`flex-1`} />

        <ButtonAsLink
          className={tw`shrink-0 px-2`}
          from='/'
          params={{
            endpointIdCan: Ulid.construct(endpointId).toCanonical(),
            exampleIdCan: Ulid.construct(exampleId).toCanonical(),
            workspaceIdCan,
          }}
          to='/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan'
          variant='ghost'
        >
          <FiExternalLink className={tw`size-4 text-slate-500`} />
          Open API
        </ButtonAsLink>

        <div className={tw`mr-3 ml-2 h-5 w-px shrink-0 bg-slate-300`} />

        <TooltipTrigger delay={750}>
          <ButtonAsLink className={tw`p-1`} from='/' search={(_) => ({ ..._, node: undefined })} to='.' variant='ghost'>
            <FiX className={tw`size-5 text-slate-500`} />
          </ButtonAsLink>
          <Tooltip className={tw`rounded-md bg-slate-800 px-2 py-1 text-xs text-white`}>Close</Tooltip>
        </TooltipTrigger>
      </div>

      <div className={tw`m-5 mb-4`}>{renderEndpointUrlForm}</div>

      <div className={tw`mx-5 overflow-auto rounded-lg border border-slate-200`}>
        <div
          className={tw`
            border-b border-slate-200 bg-slate-50 px-3 py-2 text-md leading-5 font-medium tracking-tight text-slate-800
          `}
        >
          Request
        </div>

        <ReferenceContext value={{ exampleId, nodeId, workspaceId }}>
          <EndpointRequestView
            className={tw`p-5 pt-3`}
            deltaExampleId={deltaExampleId}
            exampleId={exampleId}
            isReadOnly={isReadOnly}
          />
        </ReferenceContext>
      </div>

      {lastResponseId && (
        <div className={tw`mx-5 my-4 overflow-auto rounded-lg border border-slate-200`}>
          <div
            className={tw`
              border-b border-slate-200 bg-slate-50 px-3 py-2 text-md leading-5 font-medium tracking-tight
              text-slate-800
            `}
          >
            Response
          </div>

          <Suspense
            fallback={
              <div className={tw`flex h-full items-center justify-center p-4`}>
                <Spinner className={tw`size-8`} />
              </div>
            }
          >
            <ResponseTabs className={tw`p-5 pt-3`} responseId={lastResponseId} />
          </Suspense>
        </div>
      )}
    </>
  );
};
