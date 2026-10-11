import type { Dialog } from 'electron';

declare const PUBLIC_ENV: object;

declare global {
  interface StresseurAccount {
    user: { email: string; name?: string };
    workspace: { name: string; slug: string };
  }

  type StresseurSignInEvent =
    | { account: StresseurAccount; kind: 'signed-in' }
    | { kind: 'failed'; message: string }
    | { kind: 'waiting'; url: string; userCode: string };

  interface Window {
    electron: {
      /** Missing outside the desktop app. */
      account?: {
        cancel: () => void;
        get: () => Promise<null | StresseurAccount>;
        onEvent: (callback: (event: StresseurSignInEvent) => void) => () => void;
        open: () => void;
        signIn: () => void;
        signOut: () => Promise<void>;
      };
      dialog: <T extends keyof Dialog>(method: T, ...options: Parameters<Dialog[T]>) => Promise<ReturnType<Dialog[T]>>;
    };
  }
}
