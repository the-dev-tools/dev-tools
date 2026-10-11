import { useState } from 'react';

// localStorage for per-device conveniences (collapsed sections, last used values).
// Storage can be missing or blocked, so every access falls back quietly.

export const readStored = (key: string) => {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
};

export const writeStored = (key: string, value: string) => {
  try {
    localStorage.setItem(key, value);
  } catch {
    // A convenience; losing it is fine.
  }
};

/** A boolean kept in localStorage under `key`. */
export const useStoredFlag = (key: string, fallback: boolean) => {
  const [value, setValue] = useState(() => {
    const raw = readStored(key);
    return raw === null ? fallback : raw === 'true';
  });

  const set = (next: boolean) => {
    writeStored(key, String(next));
    setValue(next);
  };

  return [value, set] as const;
};
