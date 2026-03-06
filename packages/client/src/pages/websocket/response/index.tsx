import { useQuery } from '@tanstack/react-query';
import CodeMirror from '@uiw/react-codemirror';
import { useEffect, useRef, useState } from 'react';
import { FiArrowDown, FiArrowUp, FiTrash2 } from 'react-icons/fi';
import { twJoin } from 'tailwind-merge';
import { Button } from '@the-dev-tools/ui/button';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useTheme } from '@the-dev-tools/ui/theme';
import { guessLanguage, prettierFormatQueryOptions, useCodeMirrorLanguageExtensions } from '~/features/expression';

import { type ConnectionState, type WsMessage } from '../use-websocket';

export interface WebSocketMessageLogProps {
  clearMessages: () => void;
  error: string | undefined;
  messages: WsMessage[];
  state: ConnectionState;
}

const statusConfig = {
  connected: { color: tw`bg-success`, label: 'Connected' },
  connecting: { color: tw`bg-info`, label: 'Connecting...' },
  disconnected: { color: tw`bg-neutral-high`, label: 'Disconnected' },
  error: { color: tw`bg-danger`, label: 'Error' },
} as const;

const formatTimestamp = (ts: number) => {
  const d = new Date(ts);
  return `${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}:${d.getSeconds().toString().padStart(2, '0')}.${d.getMilliseconds().toString().padStart(3, '0')}`;
};

export const WebSocketMessageLog = ({ clearMessages, error, messages, state }: WebSocketMessageLogProps) => {
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages.length]);

  const { color, label } = statusConfig[state];

  return (
    <div className={tw`flex h-full flex-col`}>
      <div className={tw`flex items-center gap-3 border-b border-neutral px-4 py-2`}>
        <div className={tw`flex items-center gap-2`}>
          <div className={twJoin(tw`size-2 rounded-full`, color)} />
          <span className={tw`text-sm font-medium text-on-neutral`}>{label}</span>
        </div>

        {error && <span className={tw`text-xs text-danger`}>{error}</span>}

        <div className={tw`flex-1`} />

        {messages.length > 0 && <span className={tw`text-xs text-on-neutral-low`}>{messages.length} messages</span>}

        <Button className={tw`p-1`} isDisabled={messages.length === 0} onPress={clearMessages} variant='ghost'>
          <FiTrash2 className={tw`size-3.5 text-on-neutral-low`} />
        </Button>
      </div>

      <div className={tw`flex-1 overflow-auto`}>
        {messages.length === 0 ? (
          <div className={tw`flex h-full items-center justify-center text-sm text-on-neutral-low`}>
            No messages yet. Connect to start.
          </div>
        ) : (
          <div className={tw`flex flex-col`}>
            {messages.map((msg) => (
              <MessageRow key={msg.id} message={msg} />
            ))}
            <div ref={bottomRef} />
          </div>
        )}
      </div>
    </div>
  );
};

interface MessageRowProps {
  message: WsMessage;
}

const MessageRow = ({ message }: MessageRowProps) => {
  const [expanded, setExpanded] = useState(false);
  const isSent = message.direction === 'sent';
  const isJson = isJsonString(message.data);

  const preview = message.data.length > 120 ? message.data.slice(0, 120) + '...' : message.data;

  return (
    <div className={tw`border-b border-neutral-lower px-4 py-2`}>
      <button
        className={tw`flex w-full cursor-pointer items-start gap-2 text-left`}
        onClick={() => void setExpanded(!expanded)}
        type='button'
      >
        <div className={tw`mt-0.5 shrink-0`}>
          {isSent ? (
            <FiArrowUp className={tw`size-3.5 text-accent`} />
          ) : (
            <FiArrowDown className={tw`size-3.5 text-success`} />
          )}
        </div>

        <span className={tw`shrink-0 font-mono text-xs text-on-neutral-low`}>{formatTimestamp(message.timestamp)}</span>

        {!expanded && <span className={tw`min-w-0 flex-1 truncate font-mono text-xs text-on-neutral`}>{preview}</span>}
      </button>

      {expanded && (
        <div className={tw`mt-2 ml-6 overflow-auto rounded border border-neutral-lower`}>
          {isJson ? (
            <JsonViewer data={message.data} />
          ) : (
            <pre className={tw`p-3 font-mono text-xs whitespace-pre-wrap text-on-neutral`}>{message.data}</pre>
          )}
        </div>
      )}
    </div>
  );
};

const isJsonString = (str: string): boolean => {
  try {
    JSON.parse(str);
    return true;
  } catch {
    return false;
  }
};

interface JsonViewerProps {
  data: string;
}

const JsonViewer = ({ data }: JsonViewerProps) => {
  const { theme } = useTheme();
  const language = guessLanguage(data);
  const result = useQuery(prettierFormatQueryOptions({ language, text: data }));
  const extensions = useCodeMirrorLanguageExtensions(language);

  return (
    <CodeMirror
      extensions={extensions}
      height='auto'
      indentWithTab={false}
      maxHeight='300px'
      readOnly
      theme={theme}
      value={result.data}
    />
  );
};
