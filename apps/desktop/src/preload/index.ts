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
