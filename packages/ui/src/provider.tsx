import { Option } from 'effect';
import { ReactNode } from 'react';
import * as RAC from 'react-aria-components';
import { ThemeProvider } from './theme';
import { ToastQueue, ToastQueueContext } from './toast';

export interface UiProviderProps {
  children: ReactNode;
  toastQueue?: ToastQueue;
}

export const UiProvider = ({ children, toastQueue }: UiProviderProps) => {
  let _ = <RAC.RouterProvider navigate={() => undefined}>{children}</RAC.RouterProvider>;
  _ = <ToastQueueContext.Provider value={Option.fromNullable(toastQueue)}>{_}</ToastQueueContext.Provider>;
  _ = <ThemeProvider>{_}</ThemeProvider>;
  return _;
};
