import type { Metadata } from 'next';

import './styles.css';
import '@the-dev-tools/ui/fonts';

export const metadata: Metadata = {
  title: 'The Dev Tools',
};

const RootLayout = ({ children }: React.PropsWithChildren) => (
  <html lang='en'>
    <body className='font-sans'>{children}</body>
  </html>
);

export default RootLayout;
