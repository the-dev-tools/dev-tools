/// <reference types="vite/client" />

import { type Dialog } from 'electron';
import { type ProgressInfo } from 'electron-updater';

declare global {
  interface Window {
    electron: {
      dialog: <T extends keyof Dialog>(method: T, ...options: Parameters<Dialog[T]>) => Promise<ReturnType<Dialog[T]>>;
      onClose: (callback: () => void) => void;
      onCloseDone: () => void;

      update: {
        check: () => Promise<string | undefined>;
        finish: () => void;
        start: () => void;

        onProgress: (callback: (info: ProgressInfo) => void) => void;
      };
    };
  }
}
