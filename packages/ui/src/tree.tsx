import { type CollectionProps } from '@react-aria/collections';
import {
  TreeItem as AriaTreeItem,
  TreeItemContent as AriaTreeItemContent,
  type TreeItemContentProps as AriaTreeItemContentProps,
  type TreeItemProps as AriaTreeItemProps,
  Collection,
  composeRenderProps,
} from 'react-aria-components';
import { IconBaseProps } from 'react-icons';
import { twJoin, twMerge } from 'tailwind-merge';
import { tv } from 'tailwind-variants';

import { Button, ButtonProps } from './button';
import { isFocusVisibleRingStyles } from './focus-ring';
import { ChevronSolidDownIcon, Spinner } from './icons';
import { type MixinProps, splitProps } from './mixin-props';
import { tw } from './tailwind-literal';
import { composeRenderPropsTV, composeRenderPropsTW } from './utils';

// TODO: Implement drag and drop for re-ordering. Either wait for React Aria to
// potentially implement it, or switch to React Arborist

// Item root

export const treeItemRootStyles = tv({
  base: tw`text-md outline-hidden cursor-pointer select-none rounded-md bg-transparent px-3 py-1.5 font-medium leading-5 tracking-tight text-slate-800`,
  extend: isFocusVisibleRingStyles,
  variants: {
    isActive: { true: tw`bg-slate-200` },
    isHovered: { true: tw`bg-slate-100` },
    isPressed: { true: tw`bg-slate-200` },
    isSelected: { true: tw`bg-slate-200` },
  },
});

export interface TreeItemRootProps extends AriaTreeItemProps {
  isActive?: boolean;
}

export const TreeItemRoot = ({ className, isActive, ...props }: TreeItemRootProps) => (
  <AriaTreeItem {...props} className={composeRenderPropsTV(className, treeItemRootStyles, { isActive })} />
);

// Item wrapper

export interface TreeItemWrapperProps extends React.ComponentProps<'div'> {
  level: number;
}

export const TreeItemWrapper = ({ className, level, style, ...props }: TreeItemWrapperProps) => (
  <div
    {...props}
    className={twMerge(tw`flex items-center gap-2`, className)}
    style={{ paddingInlineStart: ((level - 1) * (20 / 16)).toString() + 'rem', ...style }}
  />
);

// Item mix

export interface TreeItemProps<T extends object>
  extends MixinProps<'content', Omit<AriaTreeItemContentProps, 'children'>>,
    MixinProps<'wrapper', Omit<TreeItemWrapperProps, 'level'>>,
    MixinProps<'expandButton', Omit<ButtonProps, 'children'>>,
    MixinProps<'expandIndicator', IconBaseProps>,
    MixinProps<'child', Omit<CollectionProps<T>, 'children'>>,
    Omit<TreeItemRootProps, 'children'> {
  childItem?: CollectionProps<T>['children'];
  children?: AriaTreeItemContentProps['children'];
  expandButtonIsForced?: boolean;
  loading?: boolean;
}

export const TreeItem = <T extends object>({
  childItem,
  children,
  expandButtonClassName,
  expandButtonIsForced,
  loading,
  ...mixProps
}: TreeItemProps<T>) => {
  const props = splitProps(mixProps, 'content', 'wrapper', 'expandButton', 'expandIndicator', 'child');
  return (
    <TreeItemRoot {...props.rest}>
      <AriaTreeItemContent {...props.content}>
        {composeRenderProps(children, (children, { hasChildItems, isExpanded, level }) => (
          <TreeItemWrapper level={level} {...props.wrapper}>
            {loading ? (
              <Button className={tw`p-1`} isDisabled variant='ghost'>
                <Spinner className={tw`size-3`} />
              </Button>
            ) : hasChildItems || expandButtonIsForced ? (
              <Button
                className={composeRenderPropsTW(expandButtonClassName, tw`p-1`)}
                slot='chevron'
                variant='ghost'
                {...props.expandButton}
              >
                <ChevronSolidDownIcon
                  {...props.expandIndicator}
                  className={twJoin(
                    tw`size-3 text-slate-500 transition-transform`,
                    !isExpanded ? tw`rotate-0` : tw`rotate-90`,
                    props.expandIndicator.className,
                  )}
                />
              </Button>
            ) : (
              <div />
            )}
            {children}
          </TreeItemWrapper>
        ))}
      </AriaTreeItemContent>
      {!!childItem && <Collection {...props.child}>{childItem}</Collection>}
    </TreeItemRoot>
  );
};
