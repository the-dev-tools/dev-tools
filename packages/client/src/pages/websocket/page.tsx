import { Panel, Group as PanelGroup, useDefaultLayout } from 'react-resizable-panels';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { ReferenceContext } from '~/features/expression';
import { routes } from '~/shared/routes';

import { WebSocketRequestPanel, WebSocketTopBar, WebSocketUrlBar } from './request';
import { WebSocketMessageLog } from './response';
import { useWebSocket } from './use-websocket';

export const WebSocketPage = () => {
  const { websocketId } = routes.dashboard.workspace.websocket.route.useRouteContext();
  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();

  const ws = useWebSocket();

  const endpointLayout = useDefaultLayout({ id: 'websocket-endpoint' });

  return (
    <PanelGroup {...endpointLayout} orientation='vertical'>
      <Panel className='flex h-full flex-col' id='request'>
        <ReferenceContext value={{ websocketId, workspaceId }}>
          <WebSocketTopBar websocketId={websocketId} />

          <WebSocketUrlBar
            connectionState={ws.state}
            onConnect={ws.connect}
            onDisconnect={ws.disconnect}
            websocketId={websocketId}
          />

          <WebSocketRequestPanel connectionState={ws.state} onSend={ws.send} websocketId={websocketId} />
        </ReferenceContext>
      </Panel>

      <PanelResizeHandle direction='vertical' />

      <Panel defaultSize='40%' id='messages'>
        <WebSocketMessageLog
          clearMessages={ws.clearMessages}
          error={ws.error}
          messages={ws.messages}
          state={ws.state}
        />
      </Panel>
    </PanelGroup>
  );
};
