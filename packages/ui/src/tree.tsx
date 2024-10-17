import { type CollectionProps } from '@react-aria/collections';
import { Struct } from 'effect';
import {
  UNSTABLE_Tree as AriaTree,
  UNSTABLE_TreeItem as AriaTreeItem,
  UNSTABLE_TreeItemContent as AriaTreeItemContent,
  Collection,
  composeRenderProps,
  type TreeItemContentProps as AriaTreeItemContentProps,
  type TreeItemProps as AriaTreeItemProps,
  type TreeProps as AriaTreeProps,
} from 'react-aria-components';
import { IconBaseProps } from 'react-icons';
import { LuChevronRight } from 'react-icons/lu';
import { twJoin } from 'tailwind-merge';
import { tv, VariantProps } from 'tailwind-variants';

import { splitProps, type MixinProps } from '@the-dev-tools/utils/mixin-props';

import { Button, ButtonProps, buttonStyles } from './button';
import { tw } from './tailwind-literal';
import { composeRenderPropsTW } from './utils';

// TODO: Implement drag and drop for re-ordering. Either wait for React Aria to
// potentially implement it, or switch to React Arborist

// Root

export interface TreeProps<T extends object> extends AriaTreeProps<T> {}

export const Tree = <T extends object>({ className, ...props }: TreeProps<T>) => (
  <AriaTree {...props} className={composeRenderPropsTW(className, tw`flex flex-col gap-2`)} />
);

// Item root

export interface TreeItemRootProps extends AriaTreeItemProps {}

export const TreeItemRoot = ({ className, ...props }: TreeItemRootProps) => (
  <AriaTreeItem
    {...props}
    className={composeRenderPropsTW(className, tw`group cursor-pointer select-none outline-none`)}
  />
);

// Item wrapper

export const treeItemWrapperStyles = tv({
  extend: buttonStyles,
  base: tw`flex items-center !gap-2 p-1`,
  variants: {
    isSelected: { true: tw`!bg-neutral-400` },
  },
  defaultVariants: {
    kind: 'placeholder',
    variant: 'placeholder',
  },
});

export interface TreeItemWrapperProps extends React.ComponentProps<'div'>, VariantProps<typeof treeItemWrapperStyles> {
  level: number;
}

export const TreeItemWrapper = ({ className, style, level, ...props }: TreeItemWrapperProps) => {
  const forwardedProps = Struct.omit(props, ...treeItemWrapperStyles.variantKeys);
  const variantProps = Struct.pick(props, ...treeItemWrapperStyles.variantKeys);
  return (
    <div
      {...forwardedProps}
      style={{ marginInlineStart: (level - 1).toString() + 'rem', ...style }}
      className={treeItemWrapperStyles({ ...variantProps, className })}
    />
  );
};

// Item mix

export interface TreeItemProps<T extends object>
  extends Omit<TreeItemRootProps, 'children'>,
    MixinProps<'content', Omit<AriaTreeItemContentProps, 'children'>>,
    MixinProps<'wrapper', Omit<TreeItemWrapperProps, 'level'>>,
    MixinProps<'expandButton', Omit<ButtonProps, 'children'>>,
    MixinProps<'expandIndicator', IconBaseProps>,
    MixinProps<'child', Omit<CollectionProps<T>, 'children'>> {
  children?: AriaTreeItemContentProps['children'];
  childItem?: CollectionProps<T>['children'];
  expandButtonIsForced?: boolean;
}

export const TreeItem = <T extends object>({
  children,
  childItem,
  expandButtonIsForced,
  ...mixProps
}: TreeItemProps<T>) => {
  const props = splitProps(mixProps, 'content', 'wrapper', 'expandButton', 'expandIndicator', 'child');
  return (
    <TreeItemRoot {...props.rest}>
      <AriaTreeItemContent {...props.content}>
        {composeRenderProps(children, (children, renderProps) => (
          <TreeItemWrapper
            {...Struct.pick(renderProps, 'level', ...treeItemWrapperStyles.variantKeys)}
            {...props.wrapper}
          >
            {(renderProps.hasChildRows || expandButtonIsForced) && (
              <Button kind='placeholder' variant='placeholder ghost' slot='chevron' {...props.expandButton}>
                <LuChevronRight
                  {...props.expandIndicator}
                  className={twJoin(
                    tw`transition-transform`,
                    !renderProps.isExpanded ? tw`rotate-0` : tw`rotate-90`,
                    props.expandIndicator.className,
                  )}
                />
              </Button>
            )}
            {children}
          </TreeItemWrapper>
        ))}
      </AriaTreeItemContent>
      {!!childItem && <Collection {...props.child}>{childItem}</Collection>}
    </TreeItemRoot>
  );
};
