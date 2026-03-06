import { useLiveQuery } from '@tanstack/react-db';
import { useState } from 'react';
import { WebSocketCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/web_socket';
import { Button } from '@the-dev-tools/ui/button';
import { Separator } from '@the-dev-tools/ui/separator';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { ReferenceField } from '~/features/expression';
import { useApiCollection } from '~/shared/api';
import { eqStruct, pick } from '~/shared/lib';

import { type ConnectionState } from '../use-websocket';

export interface WebSocketUrlBarProps {
  connectionState: ConnectionState;
  onConnect: (url: string) => void;
  onDisconnect: () => void;
  websocketId: Uint8Array;
}

export const WebSocketUrlBar = ({ connectionState, onConnect, onDisconnect, websocketId }: WebSocketUrlBarProps) => {
  const collection = useApiCollection(WebSocketCollectionSchema);

  const { data } = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where(eqStruct({ websocketId }))
        .select((_) => pick(_.item, 'url'))
        .findOne(),
    [collection, websocketId],
  );

  const url = data?.url ?? '';

  const [urlState, setUrlState] = useState<string>();

  const saveUrl = () => {
    if (urlState !== undefined && urlState !== url) {
      collection.utils.update({ url: urlState, websocketId });
    }
    setUrlState(undefined);
  };

  const handleConnect = () => {
    const currentUrl = urlState ?? url;
    if (urlState !== undefined) {
      collection.utils.update({ url: urlState, websocketId });
      setUrlState(undefined);
    }
    onConnect(currentUrl);
  };

  const isConnected = connectionState === 'connected';
  const isConnecting = connectionState === 'connecting';

  return (
    <div className={tw`flex gap-3 p-6 pb-0`}>
      <div className={tw`flex flex-1 items-center gap-3 rounded-lg border border-neutral px-3 py-2 shadow-xs`}>
        <span className={tw`shrink-0 rounded bg-accent-lowest px-1.5 py-0.5 text-xs font-semibold text-accent`}>
          WS
        </span>

        <Separator className={tw`h-7 shrink-0`} orientation='vertical' />

        <ReferenceField
          aria-label='URL'
          className={tw`min-w-0 flex-1 border-none font-medium tracking-tight`}
          kind='StringExpression'
          onBlur={() => void saveUrl()}
          onChange={(_) => void setUrlState(_)}
          value={urlState ?? url}
        />
      </div>

      {isConnected ? (
        <Button className={tw`px-6`} onPress={onDisconnect} variant='ghost'>
          <span className={tw`text-danger`}>Disconnect</span>
        </Button>
      ) : (
        <Button
          className={tw`px-6`}
          isDisabled={!(urlState ?? url)}
          isPending={isConnecting}
          onPress={handleConnect}
          variant='primary'
        >
          Connect
        </Button>
      )}
    </div>
  );
};
