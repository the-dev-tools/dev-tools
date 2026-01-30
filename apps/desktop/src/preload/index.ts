// See the Electron documentation for details on how to use preload scripts:
// https://www.electronjs.org/docs/latest/tutorial/process-model#preload-scripts
import { contextBridge, Dialog, ipcRenderer } from 'electron';
import { ProgressInfo } from 'electron-updater';

contextBridge.exposeInMainWorld('electron', {
  dialog: <T extends keyof Dialog>(method: T, ...options: Parameters<Dialog[T]>) =>
    ipcRenderer.invoke('dialog', method, options),
  onClose: (callback: () => void) => ipcRenderer.on('on-close', callback),
  onCloseDone: () => void ipcRenderer.send('on-close-done'),

  update: {
    check: () => ipcRenderer.invoke('update:check'),
    finish: () => void ipcRenderer.send('update:finish'),
    start: () => void ipcRenderer.send('update:start'),

    onProgress: (callback: (info: ProgressInfo) => void) =>
      ipcRenderer.on('update:progress', (_, info) => void callback(info as ProgressInfo)),
  },
});
