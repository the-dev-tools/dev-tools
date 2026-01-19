import type { Dialog } from 'electron';

declare const PUBLIC_ENV: object;

declare global {
  interface Window {
    electron: {
      dialog: <T extends keyof Dialog>(method: T, ...options: Parameters<Dialog[T]>) => Promise<ReturnType<Dialog[T]>>;
    };
  }
}
