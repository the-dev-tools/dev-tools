import {
  Modal as AriaModal,
  ModalOverlay as AriaModalOverlay,
  ModalOverlayProps as AriaModalOverlayProps,
} from 'react-aria-components';
import { tv } from 'tailwind-variants';

import { tw } from './tailwind-literal';
import { composeRenderPropsTV, composeRenderPropsTW } from './utils';

const overlayStyles = tv({
  base: tw`fixed inset-0 z-20 flex h-[--visual-viewport-height] items-center justify-center bg-slate-800/50`,
  variants: {
    isEntering: { true: tw`duration-200 ease-out animate-in fade-in` },
    isExiting: { true: tw`duration-200 ease-in animate-out fade-out` },
  },
});

export interface ModalProps extends Omit<AriaModalOverlayProps, 'className'> {
  overlayClassName?: AriaModalOverlayProps['className'];
  modalClassName?: AriaModalOverlayProps['className'];
}

export const Modal = ({ overlayClassName, modalClassName, ...props }: ModalProps) => (
  <AriaModalOverlay {...props} className={composeRenderPropsTV(overlayClassName, overlayStyles)}>
    <AriaModal
      {...props}
      className={composeRenderPropsTW(
        modalClassName,
        tw`max-h-[50vh] max-w-[70vw] overflow-auto rounded-lg bg-white p-5`,
      )}
    />
  </AriaModalOverlay>
);
