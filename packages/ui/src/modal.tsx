import { ReactNode, useState } from 'react';
import * as RAC from 'react-aria-components';
import { tv, VariantProps } from 'tailwind-variants';
import { tw } from './tailwind-literal';
import { composeStyleRenderProps } from './utils';

export const modalStyles = tv({
  slots: {
    base: tw`size-full overflow-auto rounded-lg bg-surface`,

    overlay: tw`
      fixed inset-0 z-20 flex h-(--visual-viewport-height) items-center justify-center bg-overlay

      entering:animate-in entering:duration-200 entering:ease-out entering:fade-in

      exiting:animate-out exiting:duration-200 exiting:ease-in exiting:fade-out
    `,
  },
  variants: {
    size: {
      xs: { base: tw`max-h-48 max-w-96` },
      sm: { base: tw`max-h-[40vh] max-w-[40vw]` },
      md: { base: tw`max-h-[50vh] max-w-[70vw]` },
      lg: { base: tw`max-h-[75vh] max-w-[80vw]` },
    },
  },
  defaultVariants: {
    size: 'md',
  },
});

export interface ModalProps extends RAC.ModalOverlayProps, VariantProps<typeof modalStyles> {
  overlayClassName?: RAC.ModalOverlayProps['className'];
}

export const Modal = ({ children, className, overlayClassName, style, ...props }: ModalProps) => {
  const styles = modalStyles(props);
  return (
    <RAC.ModalOverlay {...props} className={composeStyleRenderProps(overlayClassName, styles.overlay)}>
      <RAC.Modal className={composeStyleRenderProps(className, styles.base)} style={style!}>
        {children}
      </RAC.Modal>
    </RAC.ModalOverlay>
  );
};

export const useProgrammaticModal = (closeAnimationDuration = 150) => {
  const [keepOpen, setKeepOpen] = useState(false); // needed for closing animation
  const [children, setChildren] = useState<ReactNode>(null);

  const isOpen = !!children && keepOpen;

  const onOpenChange = (isOpen: boolean, node?: ReactNode) => {
    if (!isOpen) {
      setKeepOpen(false);
      setTimeout(() => void setChildren(null), closeAnimationDuration);
    } else if (node) {
      setKeepOpen(true);
      setChildren(node);
    }
  };

  return { children, isOpen, onOpenChange };
};
