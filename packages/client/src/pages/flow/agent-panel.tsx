import { eq, useLiveQuery } from '@tanstack/react-db';
import * as XF from '@xyflow/react';
import { Ulid } from 'id128';
import { FormEvent, KeyboardEvent, use, useEffect, useMemo, useRef, useState } from 'react';
import { FiArrowUp, FiChevronUp, FiEdit, FiSettings, FiX } from 'react-icons/fi';
import Markdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { NodeCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { Button } from '@the-dev-tools/ui/button';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { type Message, type ToolCall, useAgentChat } from '~/features/agent';
import { type AgentProvider, useAgentProviderKey } from '~/features/agent/use-openrouter-key';
import { useApiCollection } from '~/shared/api';
import { FlowContext } from './context';
import { nodeClientCollection } from './node';

// ---------------------------------------------------------------------------
// Tool call display helpers
// ---------------------------------------------------------------------------

const TOOL_OVERRIDES: Record<string, [active: string, done: string, label: string]> = {
  FlowRunRequest: ['Running', 'Ran', 'Flow'],
  FlowStopRequest: ['Stopping', 'Stopped', 'Flow'],
};

const VERB_PAIRS: Record<string, [active: string, done: string]> = {
  Configure: ['Configuring', 'Configured'],
  Connect: ['Connecting', 'Connected'],
  Create: ['Creating', 'Created'],
  Delete: ['Deleting', 'Deleted'],
  Disconnect: ['Disconnecting', 'Disconnected'],
  Get: ['Retrieving', 'Retrieved'],
  Inspect: ['Inspecting', 'Inspected'],
  Update: ['Updating', 'Updated'],
};

const formatToolCall = (name: string, active: boolean): [verb: string, label: string] => {
  const ov = TOOL_OVERRIDES[name];
  if (ov) return [active ? ov[0] : ov[1], ov[2]];

  const words = name
    .replace(/([a-z])([A-Z])/g, '$1 $2')
    .replace(/([A-Z]+)([A-Z][a-z])/g, '$1 $2')
    .split(' ');
  const pair = VERB_PAIRS[words[0] ?? ''];
  const verb = pair ? (active ? pair[0] : pair[1]) : active ? 'Running' : 'Ran';
  const rest = (pair ? words.slice(1) : words)
    .join(' ')
    .replace(/\bHttp\b/g, 'HTTP')
    .replace(/\bJs\b/g, 'JS')
    .replace(/\s*Request$/g, '')
    .trim();
  return [verb, rest || name];
};

const getToolBrief = (args: Record<string, unknown>): null | string => {
  if (typeof args.name === 'string' && args.name) return args.name;
  if (typeof args.url === 'string' && args.url) return args.url;
  if (typeof args.key === 'string' && args.key) return args.key;
  return null;
};

const PROVIDER_OPTIONS: Record<
  AgentProvider,
  { keyLabel: string; keysUrl: string; label: string; placeholder: string }
> = {
  anthropic: {
    keyLabel: 'Anthropic API key',
    keysUrl: 'https://console.anthropic.com/settings/keys',
    label: 'Anthropic',
    placeholder: 'Paste your Anthropic key',
  },
  openai: {
    keyLabel: 'OpenAI API key',
    keysUrl: 'https://platform.openai.com/api-keys',
    label: 'OpenAI',
    placeholder: 'Paste your OpenAI key',
  },
  openrouter: {
    keyLabel: 'OpenRouter API key',
    keysUrl: 'https://openrouter.ai/keys',
    label: 'OpenRouter',
    placeholder: 'Paste your OpenRouter key',
  },
};

export const AgentPanel = () => {
  const { flowId, setAgentPanelOpen } = use(FlowContext);
  const { apiKey, provider, setApiKey, setProvider } = useAgentProviderKey();
  const selectedNodeIds = XF.useStore(
    (s) => s.nodes.filter((n) => n.selected).map((n) => n.id),
    (a, b) => a.length === b.length && a.every((id, i) => id === b[i]),
  );
  const { cancel, clearMessages, error, isLoading, messages, sendMessage, streamingContent } = useAgentChat({
    apiKey,
    flowId,
    provider,
    selectedNodeIds,
  });

  const [input, setInput] = useState('');
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const completedToolCallIds = useMemo(() => {
    const ids = new Set<string>();
    for (const message of messages) {
      if (message.role === 'tool' && message.toolCallId) {
        ids.add(message.toolCallId);
      }
    }
    return ids;
  }, [messages]);

  const activeToolMessageId = useMemo(() => {
    if (!isLoading) return null;

    for (let i = messages.length - 1; i >= 0; i--) {
      const message = messages[i]!;
      if (message.role !== 'assistant' || !message.toolCalls?.length) continue;

      const hasPendingToolCalls = message.toolCalls.some((tc) => !completedToolCallIds.has(tc.id));
      if (hasPendingToolCalls) return message.id;
    }

    return null;
  }, [completedToolCallIds, isLoading, messages]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, isLoading, streamingContent]);

  const autoResize = () => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = '0';
    el.style.height = `${el.scrollHeight}px`;
  };

  const handleSubmit = (e?: FormEvent) => {
    e?.preventDefault();
    if (!input.trim() || isLoading) return;
    void sendMessage(input.trim());
    setInput('');
    // Reset textarea height after clearing
    requestAnimationFrame(() => {
      if (textareaRef.current) {
        textareaRef.current.style.height = '';
      }
    });
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSubmit();
    }
  };

  const handleProviderChange = (nextProvider: AgentProvider) => {
    if (nextProvider === provider) return;
    clearMessages();
    setProvider(nextProvider);
  };

  return (
    <div className={tw`flex h-full flex-col overflow-hidden bg-(--surface-1) text-sm text-(--text-primary)`}>
      {/* Header */}
      <div
        className={tw`
        mx-2 mt-2 flex items-center gap-2 rounded-[4px] border border-(--border) bg-(--surface-4) px-3 py-1.5
      `}
      >
        <div
          className={tw`
          flex flex-1 items-center gap-2 truncate text-sm font-medium tracking-[0.28px] text-(--text-primary)
        `}
        >
          Agent
        </div>

        <Button
          className={tw`p-1 text-(--text-secondary) hover:bg-(--surface-5)`}
          isDisabled={messages.length === 0}
          onPress={clearMessages}
          variant='ghost'
        >
          <FiEdit className={tw`size-3.5`} />
        </Button>

        <Button
          className={tw`p-1 text-(--text-secondary) hover:bg-(--surface-5)`}
          onPress={() => void setAgentPanelOpen?.(false)}
          variant='ghost'
        >
          <FiX className={tw`size-4`} />
        </Button>
      </div>

      {apiKey ? (
        <>
          {/* Messages */}
          <div className={tw`flex-1 overflow-x-hidden overflow-y-auto px-2 pt-1 pb-2 select-text`}>
            {messages.length === 0 ? (
              <div className={tw`text-sm text-(--text-muted)`}>
                <p>Ask me to create or modify workflow nodes.</p>
                <p className={tw`mt-1 text-(--text-subtle)`}>
                  e.g. &quot;Create a JavaScript node that returns hello world&quot;
                </p>
              </div>
            ) : (
              <div className={tw`space-y-2 py-2`}>
                {messages.map((message) => (
                  <TerminalMessage
                    completedToolCallIds={completedToolCallIds}
                    isActive={message.id === activeToolMessageId}
                    key={message.id}
                    message={message}
                  />
                ))}
                {isLoading && (streamingContent ? <StreamingMessage content={streamingContent} /> : <ThinkingBlock />)}
                <div ref={messagesEndRef} />
              </div>
            )}

            {error && <div className={tw`mt-2 text-(--text-error)`}>{error}</div>}
          </div>

          {/* Input */}
          <div
            className={tw`m-2 mt-0 rounded-[4px] border border-(--border-1) bg-(--surface-4) px-2.5 py-1.5`}
            data-agent-composer
          >
            {selectedNodeIds.length > 0 && <SelectedNodesBar selectedNodeIds={selectedNodeIds} />}
            <div className={tw`flex items-end gap-2`}>
              <textarea
                className={tw`
                  max-h-[120px] min-h-[48px] flex-1 resize-none border-none bg-transparent text-sm font-medium
                  text-(--text-primary)

                  placeholder:text-(--text-muted)

                  focus:outline-none

                  disabled:text-(--text-subtle)
                `}
                disabled={isLoading}
                onChange={(e) => {
                  setInput(e.target.value);
                  autoResize();
                }}
                onKeyDown={handleKeyDown}
                placeholder='Type a message...'
                ref={textareaRef}
                rows={1}
                value={input}
              />
            </div>
            <div className={tw`flex items-center justify-between pt-1.5`}>
              <AgentSettingsPopover
                apiKey={apiKey}
                isLoading={isLoading}
                onProviderChange={handleProviderChange}
                onSubmit={setApiKey}
                provider={provider}
              />
              {isLoading ? (
                <AbortButton onClick={cancel} />
              ) : (
                <SendButton disabled={!input.trim()} onClick={handleSubmit} />
              )}
            </div>
          </div>
        </>
      ) : (
        <ApiKeyPrompt onProviderChange={handleProviderChange} onSubmit={setApiKey} provider={provider} />
      )}
    </div>
  );
};

const SelectedNodesBar = ({ selectedNodeIds }: { selectedNodeIds: string[] }) => {
  const { flowId } = use(FlowContext);
  const nodeCollection = useApiCollection(NodeCollectionSchema);

  const { data: flowNodes } = useLiveQuery(
    (_) =>
      _.from({ node: nodeCollection })
        .where((_) => eq(_.node.flowId, flowId))
        .fn.select((_) => ({
          id: Ulid.construct(_.node.nodeId).toCanonical(),
          name: _.node.name,
        })),
    [nodeCollection, flowId],
  );

  const selectedNodes = useMemo(
    () => flowNodes.filter((_) => selectedNodeIds.includes(_.id)),
    [flowNodes, selectedNodeIds],
  );

  if (selectedNodes.length === 0) return null;

  const handleDeselect = (id: string) => {
    nodeClientCollection.update(id, (_) => (_.selected = false));
  };

  const handleClearAll = () => {
    for (const id of selectedNodeIds) {
      nodeClientCollection.update(id, (_) => (_.selected = false));
    }
  };

  return (
    <div className={tw`mb-1.5 flex flex-wrap items-center gap-1.5 border-b border-(--border-1) pb-1.5`}>
      {selectedNodes.length > 5 ? (
        <div
          className={tw`
            flex items-center gap-1 rounded-md border border-(--border) bg-(--surface-5) px-1.5 py-0.5 text-xs
            font-medium text-(--text-secondary)
          `}
        >
          <span>{selectedNodes.length} nodes selected</span>
        </div>
      ) : (
        selectedNodes.map((node) => (
          <div
            className={tw`
              flex items-center gap-1 rounded-md border border-(--border) bg-(--surface-5) px-1.5 py-0.5 text-xs
              font-medium text-(--text-secondary)
            `}
            key={node.id}
          >
            <span className={tw`max-w-[120px] truncate`}>{node.name}</span>
            <button
              className={tw`rounded-sm text-(--text-muted) hover:text-(--text-primary)`}
              onClick={() => void handleDeselect(node.id)}
              type='button'
            >
              <FiX className={tw`size-3`} />
            </button>
          </div>
        ))
      )}
      {selectedNodes.length >= 2 && (
        <button
          className={tw`text-[11px] text-(--text-muted) hover:text-(--text-secondary)`}
          onClick={handleClearAll}
          type='button'
        >
          Clear all
        </button>
      )}
    </div>
  );
};

const ApiKeyPrompt = ({
  onProviderChange,
  onSubmit,
  provider,
}: {
  onProviderChange: (provider: AgentProvider) => void;
  onSubmit: (key: string) => void;
  provider: AgentProvider;
}) => {
  const [value, setValue] = useState('');
  const providerOption = PROVIDER_OPTIONS[provider];

  useEffect(() => {
    setValue('');
  }, [provider]);

  const handleSubmit = (e?: FormEvent) => {
    e?.preventDefault();
    if (!value.trim()) return;
    onSubmit(value);
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      handleSubmit();
    }
  };

  return (
    <div className={tw`flex flex-1 flex-col items-center justify-center gap-3 px-4`}>
      <div className={tw`flex w-full flex-col gap-1 rounded-[6px] border border-(--border-1) bg-(--surface-4) p-1`}>
        {(Object.keys(PROVIDER_OPTIONS) as AgentProvider[]).map((id) => (
          <button
            className={tw`
              w-full rounded-[4px] px-2 py-1 text-left text-xs font-medium transition-colors
              ${provider === id ? 'bg-(--surface-1) text-(--text-primary)' : 'text-(--text-muted) hover:text-(--text-primary)'}
            `}
            key={id}
            onClick={() => void onProviderChange(id)}
            type='button'
          >
            {PROVIDER_OPTIONS[id].label}
          </button>
        ))}
      </div>
      <div className={tw`text-center text-sm text-(--text-muted)`}>
        <p className={tw`font-medium text-(--text-primary)`}>{providerOption.label} API Key Required</p>
        <p className={tw`mt-1`}>
          Enter your{' '}
          <a
            className={tw`text-(--brand-secondary) underline`}
            href={providerOption.keysUrl}
            rel='noreferrer'
            target='_blank'
          >
            {providerOption.keyLabel}
          </a>{' '}
          to use the agent.
        </p>
      </div>
      <div className={tw`flex w-full gap-2`}>
        <input
          className={tw`
            flex-1 rounded-[4px] border border-(--border-1) bg-(--surface-4) px-2.5 py-1.5 text-sm text-(--text-primary)

            placeholder:text-(--text-muted)

            focus:border-(--brand-secondary) focus:outline-none
          `}
          onChange={(e) => void setValue(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={providerOption.placeholder}
          type='password'
          value={value}
        />
        <button
          className={tw`
            rounded-[4px] bg-(--text-primary) px-3 py-1.5 text-sm font-medium text-(--text-inverse) transition-colors

            hover:bg-(--text-secondary)

            disabled:bg-(--text-muted)
          `}
          disabled={!value.trim()}
          onClick={() => void handleSubmit()}
          type='button'
        >
          Save
        </button>
      </div>
    </div>
  );
};

const AgentSettingsPopover = ({
  apiKey,
  isLoading,
  onProviderChange,
  onSubmit,
  provider,
}: {
  apiKey: string;
  isLoading: boolean;
  onProviderChange: (provider: AgentProvider) => void;
  onSubmit: (key: string) => void;
  provider: AgentProvider;
}) => {
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState(false);
  const [popoverLeft, setPopoverLeft] = useState(0);
  const [popoverWidth, setPopoverWidth] = useState(320);
  const [value, setValue] = useState('');
  const rootRef = useRef<HTMLDivElement>(null);
  const providerOption = PROVIDER_OPTIONS[provider];

  useEffect(() => {
    setEditing(false);
    setValue('');
  }, [provider]);

  useEffect(() => {
    if (!open) return;

    const updatePopoverLayout = () => {
      const trigger = rootRef.current;
      const composer = trigger?.closest('[data-agent-composer]');
      if (!(trigger instanceof HTMLElement) || !(composer instanceof HTMLElement)) return;

      const triggerRect = trigger.getBoundingClientRect();
      const composerRect = composer.getBoundingClientRect();
      const availableWidth = Math.max(220, composerRect.width - 16);
      const nextWidth = Math.min(320, availableWidth);
      const triggerLeftWithinComposer = triggerRect.left - composerRect.left;

      const minLeft = 8 - triggerLeftWithinComposer;
      const maxLeft = composerRect.width - 8 - triggerLeftWithinComposer - nextWidth;
      const nextLeft = Math.min(Math.max(0, minLeft), maxLeft);

      setPopoverWidth(nextWidth);
      setPopoverLeft(nextLeft);
    };

    updatePopoverLayout();

    const onDocumentMouseDown = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) {
        setOpen(false);
        setEditing(false);
        setValue('');
      }
    };

    const onDocumentKeyDown = (event: globalThis.KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      setOpen(false);
      setEditing(false);
      setValue('');
    };

    const onResize = () => {
      updatePopoverLayout();
    };

    document.addEventListener('mousedown', onDocumentMouseDown);
    document.addEventListener('keydown', onDocumentKeyDown);
    window.addEventListener('resize', onResize);

    const composer = rootRef.current?.closest('[data-agent-composer]');
    const resizeObserver =
      composer instanceof HTMLElement
        ? new ResizeObserver(() => {
            updatePopoverLayout();
          })
        : null;
    if (resizeObserver && composer instanceof HTMLElement) {
      resizeObserver.observe(composer);
    }

    return () => {
      document.removeEventListener('mousedown', onDocumentMouseDown);
      document.removeEventListener('keydown', onDocumentKeyDown);
      window.removeEventListener('resize', onResize);
      resizeObserver?.disconnect();
    };
  }, [open]);

  const handleSubmit = () => {
    if (!value.trim()) return;
    onSubmit(value);
    setEditing(false);
    setValue('');
  };

  const handleProviderClick = (nextProvider: AgentProvider) => {
    onProviderChange(nextProvider);
    setEditing(false);
    setValue('');
  };

  return (
    <div className={tw`relative`} ref={rootRef}>
      <button
        aria-expanded={open}
        aria-label='Agent settings'
        aria-haspopup='dialog'
        className={tw`
          relative rounded-[4px] border border-(--border-1) bg-(--surface-5) p-1.5 text-(--text-secondary) transition-colors

          hover:text-(--text-primary)

          disabled:text-(--text-subtle)
        `}
        disabled={isLoading}
        onClick={() => void setOpen((v) => !v)}
        type='button'
      >
        <FiSettings className={tw`size-3.5`} />
        {!apiKey && <span className={tw`absolute -top-0.5 -right-0.5 size-1.5 rounded-full bg-(--status-error)`} />}
      </button>

      {open && (
        <div
          className={tw`
            absolute bottom-full z-20 mb-1.5 rounded-[6px] border border-(--border-1) bg-(--surface-4) p-2
            shadow-lg
          `}
          role='dialog'
          style={{ left: `${popoverLeft}px`, width: `${popoverWidth}px` }}
        >
          <div className={tw`mb-1 flex items-center justify-between`}>
            <span className={tw`text-xs font-medium text-(--text-primary)`}>Agent Settings</span>
            {!apiKey ? (
              <span className={tw`text-[11px] text-(--text-error)`}>Missing API key</span>
            ) : (
              <span className={tw`text-[11px] text-(--text-muted)`}>Key saved</span>
            )}
          </div>

          <div className={tw`flex w-full flex-col gap-1 rounded-[6px] border border-(--border-1) bg-(--surface-5) p-1`}>
            {(Object.keys(PROVIDER_OPTIONS) as AgentProvider[]).map((id) => (
              <button
                className={tw`
                  w-full rounded-[4px] px-2 py-1 text-left text-xs font-medium transition-colors
                  ${provider === id ? 'bg-(--surface-1) text-(--text-primary)' : 'text-(--text-muted) hover:text-(--text-primary)'}
                `}
                disabled={isLoading}
                key={id}
                onClick={() => void handleProviderClick(id)}
                type='button'
              >
                {PROVIDER_OPTIONS[id].label}
              </button>
            ))}
          </div>

          <div className={tw`mt-1.5 flex items-center gap-2`}>
            {editing || !apiKey ? (
              <>
                <input
                  className={tw`
                    flex-1 rounded-[4px] border border-(--border-1) bg-(--surface-4) px-2 py-1 text-xs text-(--text-primary)

                    placeholder:text-(--text-muted)

                    focus:border-(--brand-secondary) focus:outline-none

                    disabled:text-(--text-subtle)
                  `}
                  disabled={isLoading}
                  onChange={(e) => void setValue(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      e.preventDefault();
                      handleSubmit();
                    }
                    if (e.key === 'Escape') {
                      e.preventDefault();
                      setOpen(false);
                      setEditing(false);
                      setValue('');
                    }
                  }}
                  placeholder={providerOption.placeholder}
                  type='password'
                  value={value}
                />
                <button
                  className={tw`
                    rounded-[4px] bg-(--text-primary) px-2 py-1 text-xs font-medium text-(--text-inverse) transition-colors

                    hover:bg-(--text-secondary)

                    disabled:bg-(--text-muted)
                  `}
                  disabled={isLoading || !value.trim()}
                  onClick={handleSubmit}
                  type='button'
                >
                  Save
                </button>
              </>
            ) : (
              <>
                <a
                  className={tw`text-xs text-(--brand-secondary) underline`}
                  href={providerOption.keysUrl}
                  rel='noreferrer'
                  target='_blank'
                >
                  {providerOption.keyLabel}
                </a>
                <button
                  className={tw`text-xs text-(--brand-secondary) hover:underline`}
                  disabled={isLoading}
                  onClick={() => void setEditing(true)}
                  type='button'
                >
                  Edit key
                </button>
              </>
            )}
          </div>
        </div>
      )}
    </div>
  );
};

const StreamingIndicator = () => (
  <div className={tw`flex h-5 items-center`}>
    <div className={tw`flex space-x-0.5`}>
      <div
        className={tw`
        size-1 animate-bounce rounded-full bg-(--text-muted)

        [animation-delay:0ms]

        [animation-duration:1.2s]
      `}
      />
      <div
        className={tw`
        size-1 animate-bounce rounded-full bg-(--text-muted)

        [animation-delay:150ms]

        [animation-duration:1.2s]
      `}
      />
      <div
        className={tw`
        size-1 animate-bounce rounded-full bg-(--text-muted)

        [animation-delay:300ms]

        [animation-duration:1.2s]
      `}
      />
    </div>
  </div>
);

const ThinkingBlock = () => {
  const [expanded, setExpanded] = useState(false);
  const [elapsed, setElapsed] = useState(1);
  const startRef = useRef(Date.now());

  useEffect(() => {
    const id = setInterval(() => {
      setElapsed(Math.max(1, Math.round((Date.now() - startRef.current) / 1000)));
    }, 100);
    return () => void clearInterval(id);
  }, []);

  return (
    <div className={tw`px-1`}>
      <button
        className={tw`flex w-full items-center gap-2 text-left`}
        onClick={() => void setExpanded((v) => !v)}
        type='button'
      >
        <span className={tw`relative text-sm font-medium text-(--text-muted)`}>
          Thinking
          <span
            aria-hidden
            className={tw`pointer-events-none absolute inset-0 text-sm font-medium`}
            style={{
              animation: 'thinking-shimmer 3s ease-in-out infinite',
              background: 'linear-gradient(90deg, transparent 0%, var(--shimmer-highlight) 50%, transparent 100%)',
              backgroundClip: 'text',
              backgroundSize: '200% 100%',
              color: 'transparent',
              WebkitBackgroundClip: 'text',
            }}
          >
            Thinking
          </span>
        </span>
        <span className={tw`text-xs text-(--text-subtle)`}>{elapsed}s</span>
        <FiChevronUp className={tw`size-3 text-(--text-subtle) transition-transform ${expanded ? '' : 'rotate-180'}`} />
      </button>
      <div className={tw`overflow-hidden transition-all duration-200 ${expanded ? 'max-h-[150px]' : 'max-h-0'}`}>
        <div className={tw`pt-1`}>
          <StreamingIndicator />
        </div>
      </div>
    </div>
  );
};

const StreamingMessage = ({ content }: { content: string }) => (
  <div className={tw`min-w-0 space-y-1 px-1 text-(--text-secondary) [overflow-wrap:anywhere]`}>
    <Markdown
      components={{
        code: ({ children, className }) => {
          const isBlock = className?.startsWith('language-');
          return isBlock ? (
            <pre
              className={tw`
              my-1 overflow-x-auto rounded-[4px] border border-(--border-1) bg-(--surface-1) p-2 text-xs
              text-(--text-secondary)
            `}
            >
              <code>{children}</code>
            </pre>
          ) : (
            <code
              className={tw`
              break-all rounded border border-(--border-1) bg-(--surface-1) px-1 py-0.5 font-mono text-[0.85em]
              text-(--text-primary)
            `}
            >
              {children}
            </code>
          );
        },
        p: ({ children }) => (
          <p className={tw`mb-1.5 break-words text-sm leading-[1.4] text-(--text-primary) [overflow-wrap:anywhere]`}>
            {children}
          </p>
        ),
        pre: ({ children }) => <>{children}</>,
      }}
      remarkPlugins={[remarkGfm]}
    >
      {content}
    </Markdown>
    <StreamingIndicator />
  </div>
);

const SendButton = ({ disabled, onClick }: { disabled: boolean; onClick: () => void }) => (
  <button
    className={tw`
      flex size-[22px] items-center justify-center rounded-full bg-(--text-primary) text-(--text-inverse) transition-colors

      hover:bg-(--text-secondary)

      disabled:bg-(--text-muted)
    `}
    disabled={disabled}
    onClick={onClick}
    type='button'
  >
    <FiArrowUp className={tw`size-3.5`} />
  </button>
);

const AbortButton = ({ onClick }: { onClick: () => void }) => (
  <button
    className={tw`
      flex size-5 items-center justify-center rounded-full bg-(--text-primary) transition-colors

      hover:bg-(--text-secondary)
    `}
    onClick={onClick}
    type='button'
  >
    <svg fill='none' height='8' viewBox='0 0 8 8' width='8'>
      <rect className={tw`fill-(--text-inverse)`} height='8' rx='1' width='8' />
    </svg>
  </button>
);

const ToolCallItem = ({ isActive, toolCall: tc }: { isActive: boolean; toolCall: ToolCall }) => {
  const [verb, label] = formatToolCall(tc.name, isActive);
  const brief = getToolBrief(tc.arguments);
  const fullText = brief ? `${verb} ${label} · ${brief}` : `${verb} ${label}`;

  return (
    <div className={tw`relative w-full overflow-hidden text-sm font-medium text-ellipsis whitespace-nowrap`}>
      <span className={tw`text-(--text-primary) dark:text-(--text-tertiary)`}>{verb}</span>{' '}
      <span className={tw`text-(--text-muted)`}>{brief ? `${label} · ${brief}` : label}</span>
      {isActive && (
        <span
          aria-hidden
          className={tw`pointer-events-none absolute inset-0 overflow-hidden text-ellipsis whitespace-nowrap`}
          style={{
            animation: 'toolcall-shimmer 1.4s linear infinite',
            background: 'linear-gradient(90deg, transparent 0%, var(--shimmer-highlight) 50%, transparent 100%)',
            backgroundClip: 'text',
            backgroundSize: '200% 100%',
            color: 'transparent',
            WebkitBackgroundClip: 'text',
          }}
        >
          {fullText}
        </span>
      )}
    </div>
  );
};

const TerminalMessage = ({
  completedToolCallIds,
  isActive,
  message,
}: {
  completedToolCallIds: Set<string>;
  isActive: boolean;
  message: Message;
}) => {
  if (message.role === 'user') {
    return (
      <div className={tw`flex min-w-0 gap-2`}>
        <span className={tw`shrink-0 text-(--brand-tertiary-2)`}>&gt;</span>
        <span
          className={tw`
            min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap rounded-[4px] border border-(--border-1) bg-(--surface-4) px-2 py-1 text-sm font-medium text-(--text-primary)
          `}
          title={message.content}
        >
          {message.content}
        </span>
      </div>
    );
  }

  if (message.role === 'tool') {
    return null;
  }

  if (message.role === 'assistant' && message.toolCalls) {
    return (
      <div className={tw`space-y-1 px-1`}>
        {message.content && (
          <div className={tw`min-w-0 text-(--text-secondary) [overflow-wrap:anywhere]`}>
            <Markdown
              components={{
                code: ({ children, className }) => {
                  const isBlock = className?.startsWith('language-');
                  return isBlock ? (
                    <pre
                      className={tw`
                      my-1 overflow-x-auto rounded-[4px] border border-(--border-1) bg-(--surface-1) p-2 text-xs
                      text-(--text-secondary)
                    `}
                    >
                      <code>{children}</code>
                    </pre>
                  ) : (
                    <code
                      className={tw`
                      break-all rounded border border-(--border-1) bg-(--surface-1) px-1 py-0.5 font-mono text-[0.85em]
                      text-(--text-primary)
                    `}
                    >
                      {children}
                    </code>
                  );
                },
                p: ({ children }) => (
                  <p
                    className={tw`mb-1.5 break-words text-sm leading-[1.4] text-(--text-primary) [overflow-wrap:anywhere]`}
                  >
                    {children}
                  </p>
                ),
                pre: ({ children }) => <>{children}</>,
              }}
              remarkPlugins={[remarkGfm]}
            >
              {message.content}
            </Markdown>
          </div>
        )}
        <div className={tw`space-y-0.5`}>
          {message.toolCalls.map((tc) => (
            <ToolCallItem isActive={isActive && !completedToolCallIds.has(tc.id)} key={tc.id} toolCall={tc} />
          ))}
        </div>
      </div>
    );
  }

  if (!message.content) return null;

  return (
    <div className={tw`min-w-0 space-y-1 px-1 text-(--text-secondary) [overflow-wrap:anywhere]`}>
      <Markdown
        components={{
          a: ({ children, href }) => (
            <a
              className={tw`text-(--brand-secondary) underline hover:opacity-80`}
              href={href}
              rel='noreferrer'
              target='_blank'
            >
              {children}
            </a>
          ),
          blockquote: ({ children }) => (
            <blockquote
              className={tw`
              my-1 break-words border-l-2 border-(--border-1) bg-(--surface-4) px-2 py-1 text-(--text-tertiary) [overflow-wrap:anywhere]
            `}
            >
              {children}
            </blockquote>
          ),
          code: ({ children, className }) => {
            const isBlock = className?.startsWith('language-');
            return isBlock ? (
              <pre
                className={tw`
                my-1 overflow-x-auto rounded-[4px] border border-(--border-1) bg-(--surface-1) p-2 text-xs
                text-(--text-secondary)
              `}
              >
                <code>{children}</code>
              </pre>
            ) : (
              <code
                className={tw`
                break-all rounded border border-(--border-1) bg-(--surface-1) px-1 py-0.5 font-mono text-[0.85em]
                text-(--text-primary)
              `}
              >
                {children}
              </code>
            );
          },
          h1: ({ children }) => (
            <div className={tw`my-1 text-base font-semibold text-(--text-primary)`}>{children}</div>
          ),
          h2: ({ children }) => (
            <div className={tw`my-1 text-[15px] font-semibold text-(--text-primary)`}>{children}</div>
          ),
          h3: ({ children }) => <div className={tw`my-1 text-sm font-semibold text-(--text-primary)`}>{children}</div>,
          li: ({ children }) => (
            <li className={tw`break-words text-sm leading-[1.4] text-(--text-secondary) [overflow-wrap:anywhere]`}>
              {children}
            </li>
          ),
          ol: ({ children }) => <ol className={tw`my-1 list-decimal space-y-0.5 pl-5`}>{children}</ol>,
          p: ({ children }) => (
            <p className={tw`mb-1.5 break-words text-sm leading-[1.4] text-(--text-primary) [overflow-wrap:anywhere]`}>
              {children}
            </p>
          ),
          pre: ({ children }) => <>{children}</>,
          strong: ({ children }) => <strong className={tw`font-semibold text-(--text-primary)`}>{children}</strong>,
          table: ({ children }) => (
            <div className={tw`my-1 overflow-x-auto`}>
              <table className={tw`w-full border-collapse text-sm`}>{children}</table>
            </div>
          ),
          td: ({ children }) => (
            <td className={tw`break-words px-2 py-1 text-(--text-secondary) [overflow-wrap:anywhere]`}>{children}</td>
          ),
          th: ({ children }) => (
            <th
              className={tw`break-words px-2 py-1 text-left text-xs font-semibold text-(--text-primary) [overflow-wrap:anywhere]`}
            >
              {children}
            </th>
          ),
          thead: ({ children }) => <thead className={tw`border-b border-(--border-1)`}>{children}</thead>,
          tr: ({ children }) => <tr className={tw`border-b border-(--border-1) last:border-0`}>{children}</tr>,
          ul: ({ children }) => <ul className={tw`my-1 list-disc space-y-0.5 pl-5`}>{children}</ul>,
        }}
        remarkPlugins={[remarkGfm]}
      >
        {message.content}
      </Markdown>
    </div>
  );
};
