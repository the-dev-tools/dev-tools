import {
  Label as AriaLabel,
  LabelProps as AriaLabelProps,
  Tag as AriaTag,
  TagGroup as AriaTagGroup,
  TagGroupProps as AriaTagGroupProps,
  TagList as AriaTagList,
  TagListProps as AriaTagListProps,
  TagProps as AriaTagProps,
} from 'react-aria-components';
import { tv } from 'tailwind-variants';

import { MixinProps, splitProps } from '@the-dev-tools/utils/mixin-props';

import { isFocusVisibleRingStyles } from './focus-ring';
import { tw } from './tailwind-literal';
import { composeRenderPropsTV, composeRenderPropsTW } from './utils';

// Tag

export const tagStyles = tv({
  extend: isFocusVisibleRingStyles,
  base: tw`cursor-pointer rounded px-2 py-1.5 text-xs font-medium leading-none tracking-tight`,
  variants: {
    isSelected: {
      false: tw`bg-transparent text-slate-400`,
      true: tw`bg-white text-slate-800 shadow`,
    },
  },
});

export interface TagProps extends AriaTagProps {}

export const Tag = ({ className, ...props }: TagProps) => {
  return <AriaTag {...props} className={composeRenderPropsTV(className, tagStyles)} />;
};

// Group

export interface TagGroupProps<T>
  extends Omit<AriaTagGroupProps, 'children'>,
    MixinProps<'list', Omit<AriaTagListProps<T>, 'children'>>,
    MixinProps<'label', Omit<AriaLabelProps, 'children'>> {
  label?: AriaLabelProps['children'];
  children?: AriaTagListProps<T>['children'];
}

export const TagGroup = <T extends object>({ label, children, listClassName, ...props }: TagGroupProps<T>) => {
  const forwardedProps = splitProps(props, 'list', 'label');

  return (
    <AriaTagGroup {...forwardedProps.rest}>
      {label && <AriaLabel {...forwardedProps.label}>{label}</AriaLabel>}
      <AriaTagList
        {...forwardedProps.list}
        className={composeRenderPropsTW(listClassName, tw`flex gap-1 rounded-md bg-slate-100 p-0.5`)}
      >
        {children}
      </AriaTagList>
    </AriaTagGroup>
  );
};
