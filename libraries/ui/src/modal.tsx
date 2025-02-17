import {
  Modal as AriaModal,
  ModalOverlay as AriaModalOverlay,
  ModalOverlayProps as AriaModalOverlayProps,
} from 'react-aria-components';
import { tv, VariantProps } from 'tailwind-variants';

import { MixinProps, splitProps } from '@the-dev-tools/utils/mixin-props';

import { tw } from './tailwind-literal';
import { composeRenderPropsTV } from './utils';

const overlayStyles = tv({
  base: tw`fixed inset-0 z-20 flex h-[--visual-viewport-height] items-center justify-center bg-slate-800/50`,
  variants: {
    isEntering: { true: tw`duration-200 ease-out animate-in fade-in` },
    isExiting: { true: tw`duration-200 ease-in animate-out fade-out` },
  },
});

const modalStyles = tv({
  base: tw`size-full overflow-auto rounded-lg bg-white`,
  variants: {
    size: {
      md: tw`max-h-[50vh] max-w-[70vw]`,
      lg: tw`max-h-[75vh] max-w-[80vw]`,
    },
  },
  defaultVariants: {
    size: 'md',
  },
});

export interface ModalProps
  extends Omit<AriaModalOverlayProps, 'className'>,
    MixinProps<'modal', VariantProps<typeof modalStyles>> {
  overlayClassName?: AriaModalOverlayProps['className'];
  modalClassName?: AriaModalOverlayProps['className'];
}

export const Modal = ({ overlayClassName, modalClassName, ...props }: ModalProps) => {
  const forwardedProps = splitProps(props, 'modal');

  return (
    <AriaModalOverlay {...forwardedProps.rest} className={composeRenderPropsTV(overlayClassName, overlayStyles)}>
      <AriaModal
        {...forwardedProps.rest}
        className={composeRenderPropsTV(modalClassName, modalStyles, forwardedProps.modal)}
      />
    </AriaModalOverlay>
  );
};
