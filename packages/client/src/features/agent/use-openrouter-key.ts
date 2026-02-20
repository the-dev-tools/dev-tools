import { useCallback, useSyncExternalStore } from 'react';

export type AgentProvider = 'anthropic' | 'openai' | 'openrouter';

const PROVIDER_STORAGE_KEY = 'agent-provider';
const API_KEY_STORAGE_KEYS: Record<AgentProvider, string> = {
  anthropic: 'agent-api-key-anthropic',
  openai: 'agent-api-key-openai',
  openrouter: 'agent-api-key-openrouter',
};

const DEFAULT_PROVIDER: AgentProvider = 'openrouter';

const listeners = new Set<() => void>();

const subscribe = (cb: () => void) => {
  listeners.add(cb);
  return () => void listeners.delete(cb);
};

const normalizeProvider = (value: null | string): AgentProvider => {
  if (value === 'anthropic' || value === 'openai' || value === 'openrouter') {
    return value;
  }
  return DEFAULT_PROVIDER;
};

const getProviderSnapshot = () => normalizeProvider(localStorage.getItem(PROVIDER_STORAGE_KEY));
const getApiKeySnapshot = (provider: AgentProvider) => localStorage.getItem(API_KEY_STORAGE_KEYS[provider]) ?? '';

const notify = () => {
  for (const cb of listeners) cb();
};

export const useAgentProviderKey = () => {
  const provider = useSyncExternalStore(
    subscribe,
    getProviderSnapshot,
    () => DEFAULT_PROVIDER,
  );
  const apiKey = useSyncExternalStore(
    subscribe,
    () => getApiKeySnapshot(provider),
    () => '',
  );

  const setProvider = useCallback((provider: AgentProvider) => {
    const current = getProviderSnapshot();
    if (current === provider) return;
    localStorage.setItem(PROVIDER_STORAGE_KEY, provider);
    notify();
  }, []);

  const setApiKey = useCallback((key: string) => {
    const trimmed = key.trim();
    const storageKey = API_KEY_STORAGE_KEYS[provider];
    if (trimmed) {
      localStorage.setItem(storageKey, trimmed);
    } else {
      localStorage.removeItem(storageKey);
    }
    notify();
  }, [provider]);

  const getApiKey = useCallback((provider: AgentProvider) => getApiKeySnapshot(provider), []);

  return {
    apiKey,
    getApiKey,
    provider,
    setApiKey,
    setProvider,
  };
};

export const useOpenRouterKey = () => {
  const { apiKey, setApiKey } = useAgentProviderKey();
  return { apiKey, setApiKey };
};
