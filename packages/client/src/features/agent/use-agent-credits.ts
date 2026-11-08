import { useEffect, useSyncExternalStore } from 'react';

/**
 * Included AI credits of the signed-in Stresseur workspace (desktop only).
 * `unavailable`: outside the desktop app, signed out, offline, or an older
 * Cloud; the agent then asks for a key as before.
 */
export type AgentCredits =
  | { credits: StresseurAiCredits; kind: 'ready' }
  | { kind: 'loading' }
  | { kind: 'unavailable'; signedIn: boolean };

let state: AgentCredits = { kind: 'loading' };
let inflight: null | Promise<void> = null;
const listeners = new Set<() => void>();

const set = (next: AgentCredits) => {
  state = next;
  for (const cb of listeners) cb();
};

export const refreshAgentCredits = (): Promise<void> => {
  const cloud = typeof window === 'undefined' ? undefined : window.electron.cloud;
  if (!cloud) {
    set({ kind: 'unavailable', signedIn: false });
    return Promise.resolve();
  }
  inflight ??= cloud
    .aiCredits()
    .then((result) => {
      if (result.ok) set({ credits: result.data, kind: 'ready' });
      else
        set({
          kind: 'unavailable',
          signedIn: result.error.kind !== 'signed-out' && result.error.kind !== 'sign-in-again',
        });
    })
    .catch(() => void set({ kind: 'unavailable', signedIn: false }))
    .finally(() => void (inflight = null));
  return inflight;
};

const subscribe = (cb: () => void) => {
  listeners.add(cb);
  return () => void listeners.delete(cb);
};

export const useAgentCredits = (): AgentCredits => {
  const credits = useSyncExternalStore(
    subscribe,
    () => state,
    () => state,
  );

  useEffect(() => {
    void refreshAgentCredits();
    const refresh = () => void refreshAgentCredits();
    window.addEventListener('stresseur-account-changed', refresh);
    const off = window.electron.account?.onEvent((event) => {
      if (event.kind === 'signed-in') refresh();
    });
    return () => {
      window.removeEventListener('stresseur-account-changed', refresh);
      off?.();
    };
  }, []);

  return credits;
};

/** "$41 of $50 left this month". */
export const formatCreditsLeft = ({ limit_cents, used_cents }: StresseurAiCredits): string => {
  const left = Math.max(0, limit_cents - used_cents) / 100;
  const leftText = left >= 1 || left === 0 ? `$${Math.floor(left)}` : `$${left.toFixed(2)}`;
  return `${leftText} of $${Math.round(limit_cents / 100)} left this month`;
};

export const creditsUsedUp = (credits: StresseurAiCredits) => credits.used_cents >= credits.limit_cents;
