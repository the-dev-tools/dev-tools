import * as RAC from 'react-aria-components';
import { twMerge } from 'tailwind-merge';
import { tw } from './tailwind-literal';
import { composeTailwindRenderProps } from './utils';

// Label

export interface FieldLabelProps extends RAC.LabelProps {}

export const FieldLabel = ({ className, ...props }: FieldLabelProps) => (
  <RAC.Label
    {...props}
    className={twMerge(className, tw`flex items-center text-sm leading-5 font-medium tracking-tight text-slate-800`)}
  />
);

// Error

export interface FieldErrorProps extends RAC.FieldErrorProps {}

export const FieldError = ({ className, ...props }: FieldErrorProps) => (
  <RAC.FieldError {...props} className={composeTailwindRenderProps(className, tw`text-red-700`)} />
);
