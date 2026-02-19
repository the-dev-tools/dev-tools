import { useAtom } from '@effect-atom/atom-react';
import { createFileRoute, useRouter } from '@tanstack/react-router';
import { pipe, Record } from 'effect';
import { Form } from 'react-aria-components';
import { Button } from '@the-dev-tools/ui/button';
import { Logo } from '@the-dev-tools/ui/illustrations';
import { RouteLink } from '@the-dev-tools/ui/link';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField } from '@the-dev-tools/ui/text-field';
import { runtimeAtom } from '~/app';
import { AuthService } from '~/shared/api';
import { routes } from '~/shared/routes';
import { DashboardLayout } from '~/shared/ui';

export const Route = createFileRoute('/(dashboard)/(user)/signIn')({
  component: RouteComponent,
});

const signInAtom = runtimeAtom.fn(AuthService.signIn);

function RouteComponent() {
  const router = useRouter();

  const [{ waiting }, submit] = useAtom(signInAtom, { mode: 'promise' });

  return (
    <DashboardLayout>
      <Form
        className={tw`container mx-auto flex max-w-sm flex-col items-center gap-x-10 px-8 py-20`}
        onSubmit={async (_) => {
          _.preventDefault();
          const data = pipe(new FormData(_.currentTarget), Record.fromEntries);
          await submit(data);
        }}
      >
        <Logo className={tw`size-20`} />

        <div className={tw`mt-10 text-xl leading-6 font-semibold tracking-tight`}>Welcome to DevTools</div>
        <div className={tw`mt-1 text-md leading-5 tracking-tight text-on-neutral-low`}>
          Please enter your account details
        </div>

        <TextInputField
          className={tw`mt-6 w-full`}
          label='Email'
          name='email'
          placeholder='Enter email...'
          type='email'
        />

        <TextInputField
          className={tw`mt-6 w-full`}
          label='Password'
          name='password'
          placeholder='Enter password...'
          type='password'
        />

        <Button className={tw`mt-11 w-full py-2`} isPending={waiting} type='submit' variant='primary'>
          Login
        </Button>

        <div className={tw`mt-4 text-md leading-5 font-medium tracking-tight`}>
          {"Don't have an account? "}

          <RouteLink
            className={tw`cursor-pointer text-accent underline`}
            to={router.routesById[routes.dashboard.workspace.user.signUp.id].fullPath}
          >
            Sign Up
          </RouteLink>
        </div>
      </Form>
    </DashboardLayout>
  );
}
