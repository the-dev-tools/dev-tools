// See the Electron documentation for details on how to use preload scripts:
// https://www.electronjs.org/docs/latest/tutorial/process-model#preload-scripts
import { contextBridge, Dialog, ipcRenderer } from 'electron';
import { ProgressInfo } from 'electron-updater';

contextBridge.exposeInMainWorld('electron', {
  account: {
    cancel: () => void ipcRenderer.send('account:cancel'),
    get: () => ipcRenderer.invoke('account:get'),
    onEvent: (callback: (event: unknown) => void) => {
      const listener = (_: unknown, event: unknown) => void callback(event);
      ipcRenderer.on('account:event', listener);
      return () => void ipcRenderer.off('account:event', listener);
    },
    open: () => void ipcRenderer.send('account:open'),
    signIn: () => void ipcRenderer.send('account:sign-in'),
    signOut: () => ipcRenderer.invoke('account:sign-out') as Promise<void>,
  },

  cloud: {
    aiAbort: (id: string) => void ipcRenderer.send('cloud:ai-abort', id),
    aiCredits: () => ipcRenderer.invoke('cloud:ai-credits'),
    /** Streams Claude events to `onEvent`; resolves with the result once the stream ends. */
    aiMessages: (id: string, body: unknown, onEvent: (event: unknown) => void) =>
      new Promise((resolve) => {
        const listener = (
          _: unknown,
          eventId: unknown,
          message: { event?: unknown; kind: string; result?: unknown },
        ) => {
          if (eventId !== id) return;
          if (message.kind === 'event') return void onEvent(message.event);
          ipcRenderer.off('cloud:ai-event', listener);
          resolve(message.result);
        };
        ipcRenderer.on('cloud:ai-event', listener);
        ipcRenderer.send('cloud:ai-messages', id, body);
      }),
    apiUrl: () => ipcRenderer.invoke('cloud:api-url') as Promise<string>,
    environments: (repo: string) => ipcRenderer.invoke('cloud:environments', repo),
    file: (repo: string, path: string) => ipcRenderer.invoke('cloud:file', repo, path),
    flows: (repo: string) => ipcRenderer.invoke('cloud:flows', repo),
    open: (url: string) => void ipcRenderer.send('cloud:open', url),
    proposeChange: (repo: string, change: unknown) => ipcRenderer.invoke('cloud:propose-change', repo, change),
    repos: () => ipcRenderer.invoke('cloud:repos'),
    run: (id: string) => ipcRenderer.invoke('cloud:run', id),
    startRun: (repo: string, run: unknown) => ipcRenderer.invoke('cloud:start-run', repo, run),
  },

  localClone: {
    envValues: (repo: string, names: string[]) => ipcRenderer.invoke('local-clone:env-values', repo, names),
    get: (repo: string) => ipcRenderer.invoke('local-clone:get', repo) as Promise<null | string>,
    head: (repo: string) => ipcRenderer.invoke('local-clone:head', repo),
    pick: (repo: { full_name: string; slug: string }) =>
      ipcRenderer.invoke('local-clone:pick', { full_name: repo.full_name, slug: repo.slug }),
    read: (repo: string, path: string) => ipcRenderer.invoke('local-clone:read', repo, path),
    unlink: (repo: string) => ipcRenderer.invoke('local-clone:unlink', repo) as Promise<void>,
    write: (repo: string, path: string, text: string) => ipcRenderer.invoke('local-clone:write', repo, path, text),
  },

  record: {
    cancel: () => ipcRenderer.invoke('record:cancel') as Promise<void>,
    onEvent: (callback: (event: unknown) => void) => {
      const listener = (_: unknown, event: unknown) => void callback(event);
      ipcRenderer.on('record:event', listener);
      return () => void ipcRenderer.off('record:event', listener);
    },
    start: (options: { demo?: boolean; dir?: string; url?: string }) => ipcRenderer.invoke('record:start', options),
    stop: () => ipcRenderer.invoke('record:stop'),
  },

  demo: {
    mark: (flowIds: string[], env: Record<string, string>) => ipcRenderer.invoke('demo:mark', flowIds, env),
    runEnv: (flowId: string) => ipcRenderer.invoke('demo:run-env', flowId),
  },

  launchpad: {
    addCi: (dir: string) => ipcRenderer.invoke('launchpad:add-ci', dir),
    detect: () => ipcRenderer.invoke('launchpad:detect'),
    installSkill: (dir: string) => ipcRenderer.invoke('launchpad:install-skill', dir),
    onProgress: (callback: (event: unknown) => void) => {
      const listener = (_: unknown, event: unknown) => void callback(event);
      ipcRenderer.on('launchpad:progress', listener);
      return () => void ipcRenderer.off('launchpad:progress', listener);
    },
    openAgent: (agent: 'claude' | 'cursor', dir: string, prompt: string) =>
      ipcRenderer.invoke('launchpad:open-agent', agent, dir, prompt) as Promise<boolean>,
    openPr: (dir: string) => ipcRenderer.invoke('launchpad:open-pr', dir),
    pickFolder: (title?: string) => ipcRenderer.invoke('launchpad:pick-folder', title) as Promise<null | string>,
    readFlows: (dir: string) => ipcRenderer.invoke('launchpad:read-flows', dir),
    unwatch: () => ipcRenderer.invoke('launchpad:unwatch') as Promise<void>,
    watch: (dir: string) => ipcRenderer.invoke('launchpad:watch', dir) as Promise<boolean>,
  },

  dialog: <T extends keyof Dialog>(method: T, ...options: Parameters<Dialog[T]>) =>
    ipcRenderer.invoke('dialog', method, options),
  onClose: (callback: () => void) => ipcRenderer.on('on-close', callback),
  onCloseDone: () => void ipcRenderer.send('on-close-done'),

  server: {
    wipeAndRestart: () => ipcRenderer.invoke('server:wipe-and-restart') as Promise<void>,
  },

  update: {
    check: () => ipcRenderer.invoke('update:check'),
    finish: () => void ipcRenderer.send('update:finish'),
    start: () => void ipcRenderer.send('update:start'),

    onProgress: (callback: (info: ProgressInfo) => void) =>
      ipcRenderer.on('update:progress', (_, info) => void callback(info as ProgressInfo)),
  },

  agentLog: {
    cleanup: () => void ipcRenderer.send('agent-log:cleanup'),
    write: (fileName: string, jsonLine: string) => void ipcRenderer.send('agent-log:write', fileName, jsonLine),
  },
});
