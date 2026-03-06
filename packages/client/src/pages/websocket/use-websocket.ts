import { useCallback, useEffect, useRef, useState } from 'react';

export type ConnectionState = 'connected' | 'connecting' | 'disconnected' | 'error';

export interface WsMessage {
  data: string;
  direction: 'received' | 'sent';
  id: string;
  timestamp: number;
}

const MAX_MESSAGES = 1000;

export interface UseWebSocketReturn {
  clearMessages: () => void;
  connect: (url: string) => void;
  disconnect: () => void;
  error: string | undefined;
  messages: WsMessage[];
  send: (message: string) => void;
  state: ConnectionState;
}

export const useWebSocket = (): UseWebSocketReturn => {
  const wsRef = useRef<null | WebSocket>(null);
  const [state, setState] = useState<ConnectionState>('disconnected');
  const [messages, setMessages] = useState<WsMessage[]>([]);
  const [error, setError] = useState<string>();

  const disconnect = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }
  }, []);

  const connect = useCallback(
    (url: string) => {
      disconnect();
      setError(undefined);
      setState('connecting');

      let wsUrl = url;
      if (wsUrl.startsWith('http://')) wsUrl = 'ws://' + wsUrl.slice(7);
      else if (wsUrl.startsWith('https://')) wsUrl = 'wss://' + wsUrl.slice(8);

      try {
        const ws = new WebSocket(wsUrl);
        wsRef.current = ws;

        ws.onopen = () => {
          setState('connected');
        };

        ws.onclose = () => {
          setState('disconnected');
          wsRef.current = null;
        };

        ws.onerror = () => {
          setError('Connection failed');
          setState('error');
        };

        ws.onmessage = (event: MessageEvent) => {
          const msg: WsMessage = {
            data: typeof event.data === 'string' ? event.data : String(event.data),
            direction: 'received',
            id: crypto.randomUUID(),
            timestamp: Date.now(),
          };
          setMessages((prev) => (prev.length >= MAX_MESSAGES ? [...prev.slice(1), msg] : [...prev, msg]));
        };
      } catch {
        setError('Invalid WebSocket URL');
        setState('error');
      }
    },
    [disconnect],
  );

  const send = useCallback((message: string) => {
    const ws = wsRef.current;
    if (ws?.readyState !== WebSocket.OPEN) return;

    ws.send(message);
    const msg: WsMessage = {
      data: message,
      direction: 'sent',
      id: crypto.randomUUID(),
      timestamp: Date.now(),
    };
    setMessages((prev) => (prev.length >= MAX_MESSAGES ? [...prev.slice(1), msg] : [...prev, msg]));
  }, []);

  const clearMessages = useCallback(() => {
    setMessages([]);
  }, []);

  useEffect(() => {
    return () => {
      wsRef.current?.close();
    };
  }, []);

  return { clearMessages, connect, disconnect, error, messages, send, state };
};
