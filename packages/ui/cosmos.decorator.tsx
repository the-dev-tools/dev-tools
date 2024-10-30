import { ReactNode, StrictMode } from 'react';
import { twJoin } from 'tailwind-merge';

import { tw } from './src/tailwind-literal';

export interface RootDecoratorOptions {
  isCentered?: boolean;
}

interface RootDecoratorProps {
  children: ReactNode;
  options: RootDecoratorOptions;
}

const RootDecorator = ({ children, options: { isCentered = true } }: RootDecoratorProps) => (
  <StrictMode>
    <div className={twJoin(tw`h-full`, isCentered && tw`flex items-center justify-center`)}>{children}</div>
  </StrictMode>
);

export default RootDecorator;
