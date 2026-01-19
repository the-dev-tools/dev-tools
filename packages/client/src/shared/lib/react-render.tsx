import { TransportProvider, useTransport } from '@connectrpc/connect-query';
import { QueryClientProvider, useQueryClient } from '@tanstack/react-query';
import { pipe } from 'effect';
import { ReactNode, StrictMode, useEffect, useRef } from 'react';
import { createRoot, Root } from 'react-dom/client';

export type ReactRender = ReturnType<typeof useReactRender>;

export const useReactRender = () => {
  const queryClient = useQueryClient();
  const transport = useTransport();

  const dom = document.createElement('div');
  const rootRef = useRef<Root>(null);

  // https://github.com/facebook/react/issues/25675
  // https://stackoverflow.com/questions/73459382/react-18-async-way-to-unmount-root
  useEffect(() => {
    const createRootTimeout = setTimeout(() => {
      rootRef.current ??= createRoot(dom);
    }, 0);

    return () => {
      clearTimeout(createRootTimeout);
      const root = rootRef.current;
      rootRef.current = null;
      return void setTimeout(() => void root?.unmount(), 0);
    };
  }, [dom]);

  return (children: ReactNode) => {
    pipe(
      children,
      (_) => <QueryClientProvider client={queryClient}>{_}</QueryClientProvider>,
      (_) => <TransportProvider transport={transport}>{_}</TransportProvider>,
      (_) => <StrictMode>{_}</StrictMode>,
      (_) => rootRef.current?.render(_),
    );

    return dom;
  };
};
