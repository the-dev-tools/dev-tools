import { Option, pipe } from 'effect';
import { createContext, ReactNode, use } from 'react';
import * as RAC from 'react-aria-components';
import { FiX } from 'react-icons/fi';
import { Button } from './button';
import { tw } from './tailwind-literal';

export interface ToastContent {
  content?: ReactNode;
  title: string;
}

export interface ToastQueue extends RAC.UNSTABLE_ToastQueue<ToastContent> {}

export const ToastQueueContext = createContext(Option.none<ToastQueue>());

export const makeToastQueue = () => new RAC.UNSTABLE_ToastQueue<ToastContent>({ maxVisibleToasts: 5 });
export const useToastQueue = () => pipe(use(ToastQueueContext), Option.getOrThrow);

export const ToastRegion = () => {
  const queue = useToastQueue();

  return (
    <RAC.UNSTABLE_ToastRegion className={tw`fixed right-5 bottom-5 flex flex-col gap-2`} queue={queue}>
      {({ toast }) => (
        <RAC.UNSTABLE_Toast
          className={tw`
            flex flex-col gap-1 rounded-md border border-border bg-surface px-3 py-2 text-sm leading-5 font-medium
            tracking-tight text-fg shadow-xl
          `}
          toast={toast}
        >
          <div className={tw`flex items-center gap-3`}>
            <RAC.Text>{toast.content.title}</RAC.Text>

            <Button className={tw`p-0.5`} slot='close' variant='ghost'>
              <FiX className={tw`size-4 text-fg-muted`} />
            </Button>
          </div>

          {toast.content.content}
        </RAC.UNSTABLE_Toast>
      )}
    </RAC.UNSTABLE_ToastRegion>
  );
};
