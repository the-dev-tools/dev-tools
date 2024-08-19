import {
  Body,
  BodyProps,
  Container,
  ContainerProps,
  Head,
  HeadProps,
  Html,
  HtmlProps,
  Preview,
  PreviewProps,
  Tailwind,
  TailwindConfig,
} from '@react-email/components';
import { twMerge } from 'tailwind-merge';
import resolveConfig from 'tailwindcss/resolveConfig';

import { MixinProps, splitProps } from '@the-dev-tools/utils/mixin-props';

import tailwindConfigRaw from '../tailwind.config';

const tailwindConfig = resolveConfig(tailwindConfigRaw) as unknown as TailwindConfig;

export interface LayoutProps
  extends MixinProps<'html', Omit<HtmlProps, 'children'>>,
    MixinProps<'head', HeadProps>,
    MixinProps<'preview', Omit<PreviewProps, 'children'>>,
    MixinProps<'body', Omit<BodyProps, 'children'>>,
    MixinProps<'container', Omit<ContainerProps, 'children'>> {
  children: React.ReactNode;
  preview: PreviewProps['children'];
}

export const Layout = ({ children, preview, ...mixProps }: LayoutProps) => {
  const props = splitProps(mixProps, 'html', 'head', 'preview', 'body', 'container');
  return (
    <Html {...props.html}>
      <Head />
      <Preview {...props.preview}>{preview}</Preview>
      <Tailwind config={tailwindConfig}>
        <Body {...props.body} className={twMerge('m-auto bg-white px-2 font-sans', props.body.className)}>
          <Container
            {...props.container}
            className={twMerge(
              'mx-auto my-10 max-w-md rounded border border-solid border-gray-200 p-5',
              props.container.className,
            )}
          >
            {children}
          </Container>
        </Body>
      </Tailwind>
    </Html>
  );
};
