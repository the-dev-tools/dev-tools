import type { ReactQueryDevtools as ReactQueryDevtoolsType } from '@tanstack/react-query-devtools';
import type { TanStackRouterDevtools as TanStackRouterDevtoolsType } from '@tanstack/router-devtools';

import { Boolean } from 'effect';
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
import { Control, FieldValues } from 'react-hook-form';
import { twMerge } from 'tailwind-merge';

import { tw } from '@the-dev-tools/ui/tailwind-literal';

const ShowDevToolsContext = createContext(false);

export const DevToolsProvider = ({ children }: PropsWithChildren) => {
  const [show, setShow] = useState(false);

  useEffect(() => {
    if (import.meta.env.PROD) return;
    // @ts-expect-error function to toggle dev tools via client console
    window.toggleDevTools = () => void setShow(Boolean.not);
  }, []);

  return <ShowDevToolsContext value={show}>{children}</ShowDevToolsContext>;
};

const TanStackRouterDevToolsLazy = lazy(() =>
  import('@tanstack/router-devtools').then((_) => ({ default: _.TanStackRouterDevtools })),
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

const RHFDevToolsLazy = lazy(() => import('@hookform/devtools').then((_) => ({ default: _.DevTool })));

interface RHFDevToolsProps<T extends FieldValues> extends ComponentProps<'div'> {
  control?: Control<T>;
  id?: string;
}

export const RHFDevTools = <T extends FieldValues>({ className, control, id, ...props }: RHFDevToolsProps<T>) => {
  const show = useContext(ShowDevToolsContext);
  if (!show) return null;
  return (
    <Suspense>
      <div {...props} className={twMerge(tw`flex items-center justify-center`, className)}>
        <RHFDevToolsLazy control={control as unknown as Control} id={id} styles={{ button: { position: 'unset' } }} />
      </div>
    </Suspense>
  );
};
