import * as RAC from 'react-aria-components';
import { FieldLabel } from './field';
import { tw } from './tailwind-literal';
import { composeTailwindRenderProps } from './utils';

export interface ProgressBarProps extends RAC.ProgressBarProps {
  label?: string;
}

export const ProgressBar = ({ className, label, ...props }: ProgressBarProps) => (
  <RAC.ProgressBar
    {...props}
    className={composeTailwindRenderProps(className, 'flex flex-col gap-2 font-sans w-64 max-w-full')}
  >
    {({ percentage, valueText }) => (
      <>
        <div className={tw`flex justify-between gap-2`}>
          <FieldLabel>{label}</FieldLabel>
          <span className={tw`text-sm text-muted-foreground`}>{valueText}</span>
        </div>

        <div
          className={tw`
            relative h-2 max-w-full overflow-hidden rounded-full bg-accent outline-1 -outline-offset-1
            outline-transparent
          `}
        >
          <div className={tw`absolute top-0 h-full rounded-full bg-primary`} style={{ width: `${percentage}%` }} />
        </div>
      </>
    )}
  </RAC.ProgressBar>
);
