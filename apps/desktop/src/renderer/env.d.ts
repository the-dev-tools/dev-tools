/// <reference types="vite/client" />

interface Window {
  electron: {
    onClose: (callback: () => void) => void;
    onCloseDone: () => void;
  };
}
