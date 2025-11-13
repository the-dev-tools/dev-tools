import { ElementType } from 'react';
import { tw } from './tailwind-literal';

interface DropIndicator {
  as?: ElementType;
}

export const DropIndicatorHorizontal = ({ as: Component = 'div' }: DropIndicator) => (
  <Component className={tw`relative z-10 col-span-full h-0 w-full ring ring-violet-700`} />
);

export const DropIndicatorVertical = ({ as: Component = 'div' }: DropIndicator) => (
  <Component className={tw`relative z-10 row-span-full h-full w-0 ring ring-violet-700`} />
);
