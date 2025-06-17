import { Option, pipe } from 'effect';
import { createContext, ReactNode, use } from 'react';
import {
  UNSTABLE_Toast as AriaToast,
  UNSTABLE_ToastQueue as AriaToastQueue,
  UNSTABLE_ToastRegion as AriaToastRegion,
  Text,
} from 'react-aria-components';
import { FiX } from 'react-icons/fi';

import { Button } from './button';
import { tw } from './tailwind-literal';

export interface ToastContent {
  content?: ReactNode;
  title: string;
}

export interface ToastQueue extends AriaToastQueue<ToastContent> {}

export const ToastQueueContext = createContext(Option.none<ToastQueue>());

export const makeToastQueue = () => new AriaToastQueue<ToastContent>({ maxVisibleToasts: 5 });
export const useToastQueue = () => pipe(use(ToastQueueContext), Option.getOrThrow);

export const ToastRegion = () => {
  const queue = useToastQueue();

  return (
    <AriaToastRegion className={tw`fixed right-5 bottom-5 flex flex-col gap-2`} queue={queue}>
      {({ toast }) => (
        <AriaToast
          className={tw`
            flex flex-col gap-1 rounded-md border border-slate-200 bg-white px-3 py-2 text-sm leading-5 font-medium
            tracking-tight text-slate-800 shadow-xl
          `}
          toast={toast}
        >
          <div className={tw`flex items-center gap-3`}>
            <Text>{toast.content.title}</Text>

            <Button className={tw`p-0.5`} slot='close' variant='ghost'>
              <FiX className={tw`size-4 text-slate-500`} />
            </Button>
          </div>

          {toast.content.content}
        </AriaToast>
      )}
    </AriaToastRegion>
  );
};
