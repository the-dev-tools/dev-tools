import { count, useLiveQuery } from '@tanstack/react-db';
import CodeMirror from '@uiw/react-codemirror';
import { Suspense, useState } from 'react';
import { Tab, TabList, TabPanel, Tabs } from 'react-aria-components';
import { twMerge } from 'tailwind-merge';
import { WebSocketHeaderCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/web_socket';
import { Button } from '@the-dev-tools/ui/button';
import { Spinner } from '@the-dev-tools/ui/spinner';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useTheme } from '@the-dev-tools/ui/theme';
import { useCodeMirrorLanguageExtensions } from '~/features/expression';
import { useApiCollection } from '~/shared/api';
import { eqStruct } from '~/shared/lib';

import { type ConnectionState } from '../use-websocket';
import { WebSocketHeaderTable } from './header';

export interface WebSocketRequestPanelProps {
  connectionState: ConnectionState;
  onSend: (message: string) => void;
  websocketId: Uint8Array;
}

export const WebSocketRequestPanel = ({ connectionState, onSend, websocketId }: WebSocketRequestPanelProps) => {
  const headerCollection = useApiCollection(WebSocketHeaderCollectionSchema);

  const { headerCount = 0 } =
    useLiveQuery(
      (_) =>
        _.from({ item: headerCollection })
          .where(eqStruct({ websocketId }))
          .select((_) => ({ headerCount: count(_.item.websocketId) }))
          .findOne(),
      [headerCollection, websocketId],
    ).data ?? {};

  const tabClass = ({ isSelected }: { isSelected: boolean }) =>
    twMerge(
      tw`
        -mb-px cursor-pointer border-b-2 border-transparent py-1.5 text-md leading-5 font-medium tracking-tight
        text-on-neutral-low transition-colors
      `,
      isSelected && tw`border-b-accent text-on-neutral`,
    );

  return (
    <Tabs className={tw`flex flex-1 flex-col gap-6 overflow-auto p-6 pt-4`}>
      <TabList className={tw`flex gap-3 border-b border-neutral`}>
        <Tab className={tabClass} id='message'>
          Message
        </Tab>
        <Tab className={tabClass} id='headers'>
          Headers
          {headerCount > 0 && <span className={tw`text-xs text-success`}> ({headerCount})</span>}
        </Tab>
      </TabList>

      <Suspense
        fallback={
          <div className={tw`flex h-full items-center justify-center`}>
            <Spinner size='lg' />
          </div>
        }
      >
        <TabPanel className={tw`flex flex-1 flex-col gap-3`} id='message'>
          <MessageComposer connectionState={connectionState} onSend={onSend} />
        </TabPanel>

        <TabPanel id='headers'>
          <WebSocketHeaderTable websocketId={websocketId} />
        </TabPanel>
      </Suspense>
    </Tabs>
  );
};

interface MessageComposerProps {
  connectionState: ConnectionState;
  onSend: (message: string) => void;
}

const MessageComposer = ({ connectionState, onSend }: MessageComposerProps) => {
  const { theme } = useTheme();
  const extensions = useCodeMirrorLanguageExtensions('json');
  const [message, setMessage] = useState('');

  const isConnected = connectionState === 'connected';

  return (
    <>
      <div className={tw`flex-1 overflow-auto rounded-lg border border-neutral`}>
        <CodeMirror
          extensions={extensions}
          height='100%'
          indentWithTab={false}
          onChange={setMessage}
          placeholder='Enter message to send...'
          theme={theme}
          value={message}
        />
      </div>

      <div className={tw`flex justify-end`}>
        <Button
          isDisabled={!isConnected || !message.trim()}
          onPress={() => {
            onSend(message);
            setMessage('');
          }}
          variant='primary'
        >
          Send Message
        </Button>
      </div>
    </>
  );
};
