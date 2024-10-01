import {
  Modal as AriaModal,
  ModalOverlay as AriaModalOverlay,
  ModalOverlayProps as AriaModalOverlayProps,
} from 'react-aria-components';
import { tv } from 'tailwind-variants';

import { tw } from './tailwind-literal';
import { composeRenderPropsTV } from './utils';

const overlayStyles = tv({
  base: tw`fixed left-0 top-0 isolate z-20 flex h-[--visual-viewport-height] w-full items-center justify-center bg-black/40 p-4 text-center`,
  variants: {
    isEntering: {
      true: tw`duration-200 ease-out animate-in fade-in`,
    },
    isExiting: {
      true: tw`duration-200 ease-in animate-out fade-out`,
    },
  },
});

const modalStyles = tv({
  base: tw`max-h-[60vh] w-full max-w-[60vw] overflow-auto rounded border border-black bg-white bg-clip-padding text-left align-middle shadow-xl`,
  variants: {
    isEntering: {
      true: tw`duration-200 ease-out animate-in zoom-in-105`,
    },
    isExiting: {
      true: tw`duration-200 ease-in animate-out zoom-out-95`,
    },
  },
});

export interface ModalProps extends Omit<AriaModalOverlayProps, 'className'> {
  overlayClassName?: AriaModalOverlayProps['className'];
  modalClassName?: AriaModalOverlayProps['className'];
}

export const Modal = ({ overlayClassName, modalClassName, ...forwardedProps }: ModalProps) => (
  <AriaModalOverlay {...forwardedProps} className={composeRenderPropsTV(overlayClassName, overlayStyles)}>
    <AriaModal {...forwardedProps} className={composeRenderPropsTV(modalClassName, modalStyles)} />
  </AriaModalOverlay>
);
