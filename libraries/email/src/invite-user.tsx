import { Button, Heading, Link, Section, Text } from '@react-email/components';

import { Layout } from './layout';
import { makeGoVariables } from './utils';

const vars = makeGoVariables('WorkspaceName', 'Username', 'InvitedByUsername', 'InviteLink');

export const InviteUserEmail = () => (
  <Layout preview={`Join ${vars.WorkspaceName} on DevTools`}>
    <Heading className='mx-0 my-8 p-0 text-center text-2xl text-black'>
      Join <strong>{vars.WorkspaceName}</strong> on <strong>DevTools</strong>
    </Heading>
    <Text className='text-sm text-black'>Hello {vars.Username},</Text>
    <Text className='text-sm text-black'>
      <strong>{vars.InvitedByUsername}</strong> has invited you to the <strong>{vars.WorkspaceName}</strong> workspace
      on <strong>DevTools</strong>.
    </Text>
    <Section className='my-8 text-center'>
      <Button
        className='rounded bg-black px-5 py-3 text-center text-xs font-semibold text-white no-underline'
        href={vars.InviteLink}
      >
        Join the workspace
      </Button>
    </Section>
    <Text className='text-sm text-black'>
      Or copy and paste this URL into your browser:{' '}
      <Link href={vars.InviteLink} className='text-blue-600 no-underline'>
        {vars.InviteLink}
      </Link>
    </Text>
  </Layout>
);

export default InviteUserEmail;
