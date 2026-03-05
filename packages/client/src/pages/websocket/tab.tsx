import { useLiveQuery } from '@tanstack/react-db';
import { FiWifi } from 'react-icons/fi';
import { WebSocketCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/web_socket';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/shared/api';
import { eqStruct } from '~/shared/lib';
import { routes } from '~/shared/routes';

export interface WebSocketTabProps {
  websocketId: Uint8Array;
}

export const websocketTabId = ({ websocketId }: WebSocketTabProps) =>
  JSON.stringify({ route: routes.dashboard.workspace.websocket.route.id, websocketId });

export const WebSocketTab = ({ websocketId }: WebSocketTabProps) => {
  const websocketCollection = useApiCollection(WebSocketCollectionSchema);

  const name =
    useLiveQuery(
      (_) =>
        _.from({ item: websocketCollection })
          .where(eqStruct({ websocketId }))
          .select((_) => ({ name: _.item.name }))
          .findOne(),
      [websocketCollection, websocketId],
    ).data?.name ?? 'WebSocket';

  return (
    <>
      <FiWifi className={tw`size-3.5 text-on-neutral-low`} />
      <span className={tw`min-w-0 flex-1 truncate`}>{name}</span>
    </>
  );
};
