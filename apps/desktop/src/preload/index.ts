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
    start: (url: string) => ipcRenderer.invoke('record:start', url),
    stop: () => ipcRenderer.invoke('record:stop'),
  },

  dialog: <T extends keyof Dialog>(method: T, ...options: Parameters<Dialog[T]>) =>
    ipcRenderer.invoke('dialog', method, options),
  onClose: (callback: () => void) => ipcRenderer.on('on-close', callback),
  onCloseDone: () => void ipcRenderer.send('on-close-done'),

  renameNotice: {
    dismiss: () => void ipcRenderer.send('rename-notice:dismiss'),
    get: () => ipcRenderer.invoke('rename-notice:get') as Promise<boolean>,
  },

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
