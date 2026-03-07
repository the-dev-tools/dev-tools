import { createFileRoute } from '@tanstack/react-router';
import { openTab } from '~/widgets/tabs';
import { WebSocketPage } from '../../../page';
import { WebSocketTab, websocketTabId } from '../../../tab';

export const Route = createFileRoute(
  '/(dashboard)/(workspace)/workspace/$workspaceIdCan/(websocket)/websocket/$websocketIdCan/',
)({
  component: WebSocketPage,
  onEnter: async (match) => {
    const { websocketId } = match.context;

    await openTab({
      id: websocketTabId({ websocketId }),
      match,
      node: <WebSocketTab websocketId={websocketId} />,
    });
  },
  onStay: async (match) => {
    const { websocketId } = match.context;

    await openTab({
      id: websocketTabId({ websocketId }),
      match,
      node: <WebSocketTab websocketId={websocketId} />,
    });
  },
});
