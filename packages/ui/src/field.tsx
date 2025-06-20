import {
  FieldError as AriaFieldError,
  type FieldErrorProps as AriaFieldErrorProps,
  Label as AriaLabel,
  type LabelProps as AriaLabelProps,
} from 'react-aria-components';
import { twMerge } from 'tailwind-merge';

import { tw } from './tailwind-literal';
import { composeRenderPropsTW } from './utils';

// Label

export interface FieldLabelProps extends AriaLabelProps {}

export const FieldLabel = ({ className, ...props }: FieldLabelProps) => (
  <AriaLabel
    className={twMerge(className, tw`flex items-center text-sm leading-5 font-medium tracking-tight text-slate-800`)}
    {...props}
  />
);

// Error

export interface FieldErrorProps extends AriaFieldErrorProps {}

export const FieldError = ({ className, ...props }: FieldErrorProps) => (
  <AriaFieldError {...props} className={composeRenderPropsTW(className, tw`text-red-700`)} />
);
