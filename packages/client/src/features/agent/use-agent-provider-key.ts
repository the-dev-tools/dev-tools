import { useCallback, useSyncExternalStore } from 'react';

/** Providers that need the user's own API key. */
export type OwnKeyProvider = 'anthropic' | 'openai' | 'openrouter';

/** `stresseur`: Claude on the Stresseur workspace's included AI credits, no key. */
export type AgentProvider = 'stresseur' | OwnKeyProvider;

const PROVIDER_STORAGE_KEY = 'agent-provider';
/** Set once the user picks a provider themselves; their choice then wins over included credits. */
const CHOSEN_STORAGE_KEY = 'agent-provider-chosen';
const API_KEY_STORAGE_KEYS: Record<OwnKeyProvider, string> = {
  anthropic: 'agent-api-key-anthropic',
  openai: 'agent-api-key-openai',
  openrouter: 'agent-api-key-openrouter',
};

const DEFAULT_PROVIDER: OwnKeyProvider = 'openrouter';

export const isOwnKeyProvider = (value: unknown): value is OwnKeyProvider =>
  value === 'anthropic' || value === 'openai' || value === 'openrouter';

const listeners = new Set<() => void>();
let version = 0;

const subscribe = (cb: () => void) => {
  listeners.add(cb);
  return () => void listeners.delete(cb);
};

const notify = () => {
  version++;
  for (const cb of listeners) cb();
};

const storedKey = (provider: OwnKeyProvider) => localStorage.getItem(API_KEY_STORAGE_KEYS[provider]) ?? '';

/** The own-key provider to fall back to: the stored one, else the first with a key, else OpenRouter. */
const ownProviderSnapshot = (): OwnKeyProvider => {
  const stored = localStorage.getItem(PROVIDER_STORAGE_KEY);
  if (isOwnKeyProvider(stored)) return stored;
  return (Object.keys(API_KEY_STORAGE_KEYS) as OwnKeyProvider[]).find((_) => storedKey(_)) ?? DEFAULT_PROVIDER;
};

/**
 * Which provider the agent uses. With included credits, Stresseur is the
 * default; a provider the user chose, or an own key saved before credits
 * existed, wins over it.
 */
const resolveProvider = (included: boolean): AgentProvider => {
  const stored = localStorage.getItem(PROVIDER_STORAGE_KEY);
  if (stored === 'stresseur') return included ? 'stresseur' : ownProviderSnapshot();
  const chosen = localStorage.getItem(CHOSEN_STORAGE_KEY) === '1';
  if (isOwnKeyProvider(stored) && (chosen || storedKey(stored))) return stored;
  return included ? 'stresseur' : ownProviderSnapshot();
};

export const useAgentProviderKey = ({ included = false }: { included?: boolean } = {}) => {
  useSyncExternalStore(
    subscribe,
    () => version,
    () => version,
  );
  const provider = typeof localStorage === 'undefined' ? DEFAULT_PROVIDER : resolveProvider(included);
  const ownProvider = typeof localStorage === 'undefined' ? DEFAULT_PROVIDER : ownProviderSnapshot();
  const apiKey = provider === 'stresseur' ? '' : storedKey(provider);

  const setProvider = useCallback((next: AgentProvider) => {
    localStorage.setItem(PROVIDER_STORAGE_KEY, next);
    localStorage.setItem(CHOSEN_STORAGE_KEY, '1');
    notify();
  }, []);

  const setApiKey = useCallback(
    (key: string) => {
      if (provider === 'stresseur') return;
      const trimmed = key.trim();
      const storageKey = API_KEY_STORAGE_KEYS[provider];
      if (trimmed) {
        localStorage.setItem(storageKey, trimmed);
        // Saving a key is choosing that provider.
        localStorage.setItem(PROVIDER_STORAGE_KEY, provider);
        localStorage.setItem(CHOSEN_STORAGE_KEY, '1');
      } else {
        localStorage.removeItem(storageKey);
      }
      notify();
    },
    [provider],
  );

  const getApiKey = useCallback((provider: OwnKeyProvider) => storedKey(provider), []);

  return {
    apiKey,
    getApiKey,
    ownProvider,
    provider,
    setApiKey,
    setProvider,
  };
};

export const useOpenRouterKey = () => {
  const { apiKey, setApiKey } = useAgentProviderKey();
  return { apiKey, setApiKey };
};
