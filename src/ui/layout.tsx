import backgroundImage from 'data-base64:@/../assets/background.png';
import * as React from 'react';
import { twMerge } from 'tailwind-merge';

export interface WithBackgroundProps extends React.ComponentPropsWithoutRef<'div'> {
  innerClassName?: string;
}

export const WithBackground = ({ className, innerClassName, children, ...props }: WithBackgroundProps) => (
  <div {...props} className={twMerge('relative z-0 size-full bg-slate-50 font-sans', className)}>
    <div className='absolute inset-x-0 top-0 -z-10 bg-slate-50'>
      <img src={backgroundImage} alt='Background' className='w-full mix-blend-luminosity' />
      <div className='absolute inset-0 shadow-[inset_0_0_2rem_2rem_var(--tw-shadow-color)] shadow-slate-50' />
    </div>

    <div className={twMerge('size-full', innerClassName)}>{children}</div>
  </div>
);
