import * as RAC from 'react-aria-components';
import { focusVisibleRingStyles } from './focus-ring';
import { tw } from './tailwind-literal';
import { composeTailwindRenderProps } from './utils';

// Tag

export interface TagProps extends RAC.TagProps {}

export const Tag = ({ className, ...props }: TagProps) => (
  <RAC.Tag
    {...props}
    className={composeTailwindRenderProps(
      className,
      focusVisibleRingStyles(),
      tw`
        cursor-pointer rounded-sm bg-transparent px-2 py-1.5 text-xs leading-none font-medium tracking-tight
        text-slate-400

        selected:bg-white selected:text-slate-800 selected:shadow-sm
      `,
    )}
  />
);

// Group

export interface TagGroupProps<T>
  extends Omit<RAC.TagGroupProps, 'children'>, Omit<RAC.TagListProps<T>, 'className' | 'style'> {
  label?: RAC.LabelProps['children'];
}

export const TagGroup = <T extends object>({ children, label, ...props }: TagGroupProps<T>) => (
  <RAC.TagGroup {...props}>
    {label && <RAC.Label>{label}</RAC.Label>}
    <RAC.TagList className={tw`flex gap-1 rounded-md bg-slate-100 p-0.5`}>{children}</RAC.TagList>
  </RAC.TagGroup>
);
