import * as RAC from 'react-aria-components';
import { tv, VariantProps } from 'tailwind-variants';
import { tw } from './tailwind-literal';
import { composeStyleRenderProps } from './utils';

export const modalStyles = tv({
  slots: {
    base: tw`size-full overflow-auto rounded-lg bg-white`,

    overlay: tw`
      fixed inset-0 z-20 flex h-(--visual-viewport-height) items-center justify-center bg-slate-800/50

      entering:animate-in entering:duration-200 entering:ease-out entering:fade-in

      exiting:animate-out exiting:duration-200 exiting:ease-in exiting:fade-out
    `,
  },
  variants: {
    size: {
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

export const Modal = ({ className, overlayClassName, style, ...props }: ModalProps) => {
  const styles = modalStyles(props);
  return (
    <RAC.ModalOverlay {...props} className={composeStyleRenderProps(overlayClassName, styles.overlay)} style={style!}>
      <RAC.Modal {...props} className={composeStyleRenderProps(className, styles.base)} />
    </RAC.ModalOverlay>
  );
};
