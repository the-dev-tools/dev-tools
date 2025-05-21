// See the Electron documentation for details on how to use preload scripts:
// https://www.electronjs.org/docs/latest/tutorial/process-model#preload-scripts
import { contextBridge, Dialog, ipcRenderer } from 'electron';

contextBridge.exposeInMainWorld('electron', {
  dialog: <T extends keyof Dialog>(method: T, ...options: Parameters<Dialog[T]>) =>
    ipcRenderer.invoke('dialog', method, options),
});
