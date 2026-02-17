import { useCallback, useSyncExternalStore } from 'react';

const STORAGE_KEY = 'openrouter-api-key';

const listeners = new Set<() => void>();

const subscribe = (cb: () => void) => {
  listeners.add(cb);
  return () => void listeners.delete(cb);
};

const getSnapshot = () => localStorage.getItem(STORAGE_KEY) ?? '';

const notify = () => {
  for (const cb of listeners) cb();
};

export const useOpenRouterKey = () => {
  const apiKey = useSyncExternalStore(subscribe, getSnapshot, () => '');

  const setApiKey = useCallback((key: string) => {
    const trimmed = key.trim();
    if (trimmed) {
      localStorage.setItem(STORAGE_KEY, trimmed);
    } else {
      localStorage.removeItem(STORAGE_KEY);
    }
    notify();
  }, []);

  return { apiKey, setApiKey };
};
