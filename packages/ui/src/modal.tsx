import {
  Modal as AriaModal,
  ModalOverlay as AriaModalOverlay,
  ModalOverlayProps as AriaModalOverlayProps,
} from 'react-aria-components';
import { tv, VariantProps } from 'tailwind-variants';

import { MixinProps, splitProps } from './mixin-props';
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
  variants: {
    size: {
      lg: tw`max-h-[75vh] max-w-[80vw]`,
      md: tw`max-h-[50vh] max-w-[70vw]`,
      sm: tw`max-h-[40vh] max-w-[40vw]`,
    },
  },
  defaultVariants: {
    size: 'md',
  },
});

export interface ModalProps
  extends MixinProps<'modal', VariantProps<typeof modalStyles>>,
    Omit<AriaModalOverlayProps, 'className' | 'style'> {
  modalClassName?: AriaModalOverlayProps['className'];
  modalStyle?: AriaModalOverlayProps['style'];
  overlayClassName?: AriaModalOverlayProps['className'];
}

export const Modal = ({ modalClassName, modalStyle, overlayClassName, ...props }: ModalProps) => {
  const forwardedProps = splitProps(props, 'modal');

  return (
    <AriaModalOverlay {...forwardedProps.rest} className={composeRenderPropsTV(overlayClassName, overlayStyles)}>
      <AriaModal
        {...forwardedProps.rest}
        className={composeRenderPropsTV(modalClassName, modalStyles, forwardedProps.modal)}
        style={modalStyle ?? {}}
      />
    </AriaModalOverlay>
  );
};
