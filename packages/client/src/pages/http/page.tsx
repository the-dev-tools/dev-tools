import { eq, useLiveQuery } from '@tanstack/react-db';
import { Ulid } from 'id128';
import { useMemo } from 'react';
import { Panel, Group as PanelGroup, useDefaultLayout } from 'react-resizable-panels';
import { HttpResponseCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { ReferenceContext } from '~/features/expression';
import { useApiCollection } from '~/shared/api';
import { pick } from '~/shared/lib';
import { routes } from '~/shared/routes';
import { HttpRequestPanel, HttpTopBar } from './request';
import { ResponseInfo, ResponsePanel } from './response';

export const HttpPage = () => {
  const { httpId } = routes.dashboard.workspace.http.route.useRouteContext();
  return <Page httpId={httpId} />;
};

export const HttpDeltaPage = () => {
  const { deltaHttpId, httpId } = routes.dashboard.workspace.http.delta.useRouteContext();
  return <Page deltaHttpId={deltaHttpId} httpId={httpId} />;
};

interface PageProps {
  deltaHttpId?: Uint8Array;
  httpId: Uint8Array;
}

const Page = ({ deltaHttpId, httpId }: PageProps) => {
  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();

  const responseCollection = useApiCollection(HttpResponseCollectionSchema);

  const { data: responses } = useLiveQuery(
    (_) =>
      _.from({ item: responseCollection })
        .where((_) => eq(_.item.httpId, deltaHttpId ?? httpId))
        .select((_) => pick(_.item, 'httpResponseId')),
    [responseCollection, deltaHttpId, httpId],
  );

  // Find the latest response by ULID canonical string comparison instead of
  // raw Uint8Array to avoid incorrect JS string coercion ordering.
  const httpResponseId = useMemo(() => {
    if (responses.length === 0) return undefined;
    return responses.reduce((latest, curr) => {
      const latestKey = Ulid.construct(latest.httpResponseId).toCanonical();
      const currKey = Ulid.construct(curr.httpResponseId).toCanonical();
      return currKey > latestKey ? curr : latest;
    }).httpResponseId;
  }, [responses]);

  const endpointLayout = useDefaultLayout({ id: 'endpoint' });

  return (
    <PanelGroup {...endpointLayout} orientation='vertical'>
      <Panel className='flex h-full flex-col' id='request'>
        <ReferenceContext value={{ httpId, workspaceId, ...(deltaHttpId && { deltaHttpId }) }}>
          <HttpTopBar deltaHttpId={deltaHttpId} httpId={httpId} />

          <HttpRequestPanel deltaHttpId={deltaHttpId} httpId={httpId} />
        </ReferenceContext>
      </Panel>

      {httpResponseId && (
        <>
          <PanelResizeHandle direction='vertical' />

          <Panel defaultSize='40%' id='response'>
            <ResponsePanel fullWidth httpResponseId={httpResponseId}>
              <ResponseInfo httpResponseId={httpResponseId} />
            </ResponsePanel>
          </Panel>
        </>
      )}
    </PanelGroup>
  );
};
