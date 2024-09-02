import {
  FieldError as AriaFieldError,
  Label as AriaLabel,
  type FieldErrorProps as AriaFieldErrorProps,
  type LabelProps as AriaLabelProps,
} from 'react-aria-components';

import { tw } from './tailwind-literal';
import { composeRenderPropsTW } from './utils';

// Label

export interface FieldLabelProps extends AriaLabelProps {}

export const FieldLabel = (props: FieldLabelProps) => <AriaLabel {...props} />;

// Error

export interface FieldErrorProps extends AriaFieldErrorProps {}

export const FieldError = ({ className, ...props }: FieldErrorProps) => (
  <AriaFieldError {...props} className={composeRenderPropsTW(className, tw`text-red-700`)} />
);
