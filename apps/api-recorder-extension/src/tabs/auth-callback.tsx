import '~styles.css';

import type { IconType } from 'react-icons';

import { Effect, Match, Option, pipe, Tuple } from 'effect';
import * as React from 'react';
import * as FeatherIcons from 'react-icons/fi';
import { twMerge } from 'tailwind-merge';

import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { Button } from '~/ui/button';
import * as Auth from '~auth';
import { Layout } from '~layout';
import { Runtime } from '~runtime';

interface FeaturedIconProps extends React.ComponentPropsWithoutRef<'div'> {
  Icon: IconType;
  iconClassName?: string;
}

const FeaturedIcon = ({ className, Icon, iconClassName, ...props }: FeaturedIconProps) => (
  <div className={twMerge('shadow-xs rounded-lg border p-3', className)} {...props}>
    <Icon className={twMerge('size-7', iconClassName)} />
  </div>
);

const Heading = ({ children, ...props }: Omit<React.ComponentPropsWithoutRef<'h1'>, 'className'>) => (
  <h1 {...props} className='pb-3 text-2xl font-semibold leading-tight text-gray-800'>
    {children}
  </h1>
);

const Subheading = ({ children, ...props }: Omit<React.ComponentPropsWithoutRef<'h2'>, 'className'>) => (
  <h2 {...props} className='text-base leading-6 text-slate-500 *:text-indigo-600'>
    {children}
  </h2>
);

const AuthCallbackPage = () => {
  const token = new URLSearchParams(window.location.search).get('magic_credential');

  const [state, setState] = React.useState<'failure' | 'loading' | 'success'>(token ? 'loading' : 'failure');
  const [resendLoading, setResendLoading] = React.useState(false);

  const email = pipe(
    Auth.useEmail(),
    Tuple.getFirst,
    Option.flatten,
    Option.getOrElse(() => 'your email'),
  );

  React.useEffect(() => {
    if (!token) return;
    void Effect.gen(function* () {
      const success = yield* Auth.loginConfirm(token);
      setState(success ? 'success' : 'failure');
    }).pipe(Runtime.runPromise);
  }, [token]);

  const inner = Match.value(state).pipe(
    Match.when('loading', () => (
      <>
        <FeaturedIcon
          className='border-gray-200 bg-white text-slate-800'
          Icon={FeatherIcons.FiLoader}
          iconClassName='animate-spin'
        />

        <div className='text-center'>
          <Heading>Authenticating...</Heading>
          <Subheading>
            We are authenticating <span>{email}</span>
          </Subheading>
        </div>
      </>
    )),

    Match.when('success', () => (
      <>
        <FeaturedIcon className='border-green-600 bg-green-50 text-green-600' Icon={FeatherIcons.FiCheckCircle} />

        <div className='text-center'>
          <Heading>Authentication Successful!</Heading>
          <Subheading>
            We have successfully authenticated <span>{email}</span>
          </Subheading>
        </div>
      </>
    )),

    Match.when('failure', () => (
      <>
        <FeaturedIcon className='border-red-500 bg-red-50 text-red-500' Icon={FeatherIcons.FiXCircle} />

        <div className='text-center'>
          <Heading>Authentication Failed!</Heading>
          <Subheading>
            We have failed to authenticate <span>{email}</span>
          </Subheading>
        </div>

        <Button
          className='w-full'
          onPress={() =>
            Effect.gen(function* () {
              setResendLoading(true);
              const success = yield* Auth.loginInit(email);
              setResendLoading(false);
              if (success) setState('success');
            }).pipe(Runtime.runPromise)
          }
        >
          {resendLoading && <FeatherIcons.FiLoader className='animate-spin' />}
          Resend email
        </Button>
      </>
    )),

    Match.exhaustive,
  );

  return (
    <Layout innerClassName={tw`flex items-center justify-center`}>
      <div className='flex flex-col items-center gap-6'>{inner}</div>
    </Layout>
  );
};

export default AuthCallbackPage;
