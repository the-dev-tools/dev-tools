import { Suspense } from 'react';
import { Panel, PanelGroup } from 'react-resizable-panels';
import { Spinner } from '@the-dev-tools/ui/spinner';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { ReferenceContext } from '~/reference';
import { httpRouteApi, workspaceRouteApi } from '~/routes';
import { HttpRequest, HttpTopBar } from './request';

export const HttpPage = () => {
  const { httpId } = httpRouteApi.useLoaderData();
  const { workspaceId } = workspaceRouteApi.useLoaderData();

  return (
    <Suspense
      fallback={
        <div className={tw`flex h-full items-center justify-center`}>
          <Spinner size='xl' />
        </div>
      }
    >
      <PanelGroup autoSaveId='endpoint' direction='vertical'>
        <Panel className='flex h-full flex-col' defaultSize={60} id='request' order={1}>
          <ReferenceContext value={{ httpId, workspaceId }}>
            <HttpTopBar httpId={httpId} />

            <HttpRequest httpId={httpId} />
          </ReferenceContext>
        </Panel>
        <Suspense>{/* <ResponsePanel /> */}</Suspense>
      </PanelGroup>
    </Suspense>
  );
};
