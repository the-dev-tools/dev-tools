import type { ReactQueryDevtools as ReactQueryDevtoolsType } from '@tanstack/react-query-devtools';
import type { TanStackRouterDevtools as TanStackRouterDevtoolsType } from '@tanstack/react-router-devtools';
import {
  ComponentProps,
  createContext,
  lazy,
  PropsWithChildren,
  ReactNode,
  Suspense,
  useContext,
  useEffect,
  useState,
} from 'react';
import { Options as ReactScanOptions, setOptions } from 'react-scan';

const ShowDevToolsContext = createContext(false);

export const DevToolsProvider = ({ children }: PropsWithChildren) => {
  const key = 'DEV_TOOLS_ENABLED';

  const [show, setShow] = useState(!import.meta.env.PROD && Boolean(localStorage.getItem(key)));

  useEffect(() => {
    if (import.meta.env.PROD) return;
    // @ts-expect-error function to toggle dev tools via client console
    window.toggleDevTools = () => {
      if (show) localStorage.removeItem(key);
      else localStorage.setItem(key, 'true');
      setShow(!show);
    };
  }, [show]);

  return <ShowDevToolsContext value={show}>{children}</ShowDevToolsContext>;
};

const TanStackRouterDevToolsLazy = lazy(() =>
  import('@tanstack/react-router-devtools').then((_) => ({ default: _.TanStackRouterDevtools })),
);

export const TanStackRouterDevTools = (props: ComponentProps<typeof TanStackRouterDevtoolsType>) => {
  const show = useContext(ShowDevToolsContext);
  if (!show) return null;
  return (
    <Suspense>
      <TanStackRouterDevToolsLazy {...props} />
    </Suspense>
  );
};

const ReactQueryDevToolsLazy = lazy<(props: ComponentProps<typeof ReactQueryDevtoolsType>) => ReactNode>(() =>
  import('@tanstack/react-query-devtools/production').then((_) => ({ default: _.ReactQueryDevtools })),
);

export const ReactQueryDevTools = (props: ComponentProps<typeof ReactQueryDevToolsLazy>) => {
  const show = useContext(ShowDevToolsContext);
  if (!show) return null;
  return (
    <Suspense>
      <ReactQueryDevToolsLazy {...props} />
    </Suspense>
  );
};

export const ReactScanDevTools = (props: ReactScanOptions) => {
  const show = useContext(ShowDevToolsContext);

  useEffect(() => {
    setOptions({ enabled: false, showToolbar: show, ...props });
  }, [props, show]);

  return null;
};
