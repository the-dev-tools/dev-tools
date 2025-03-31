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
  base: tw`h-(--visual-viewport-height) fixed inset-0 z-20 flex items-center justify-center bg-slate-800/50`,
  variants: {
    isEntering: { true: tw`animate-in fade-in duration-200 ease-out` },
    isExiting: { true: tw`animate-out fade-out duration-200 ease-in` },
  },
});

const modalStyles = tv({
  base: tw`size-full overflow-auto rounded-lg bg-white`,
  defaultVariants: {
    size: 'md',
  },
  variants: {
    size: {
      lg: tw`max-h-[75vh] max-w-[80vw]`,
      md: tw`max-h-[50vh] max-w-[70vw]`,
      sm: tw`max-h-[40vh] max-w-[40vw]`,
    },
  },
});

export interface ModalProps
  extends MixinProps<'modal', VariantProps<typeof modalStyles>>,
    Omit<AriaModalOverlayProps, 'className'> {
  modalClassName?: AriaModalOverlayProps['className'];
  overlayClassName?: AriaModalOverlayProps['className'];
}

export const Modal = ({ modalClassName, overlayClassName, ...props }: ModalProps) => {
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
