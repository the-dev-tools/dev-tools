import { useQueryErrorResetBoundary } from '@tanstack/react-query';
import { ErrorRouteComponent, useRouter } from '@tanstack/react-router';
import { useEffect } from 'react';
import { Button } from '@the-dev-tools/ui/button';
import { tw } from '@the-dev-tools/ui/tailwind-literal';

// https://tanstack.com/router/latest/docs/framework/react/guide/external-data-loading#error-handling-with-tanstack-query
export const ErrorComponent: ErrorRouteComponent = ({ error }) => {
  const router = useRouter();
  const queryErrorResetBoundary = useQueryErrorResetBoundary();

  useEffect(() => void queryErrorResetBoundary.reset(), [queryErrorResetBoundary]);

  return (
    <div className={tw`flex h-full flex-col items-center justify-center gap-4 text-center`}>
      <div>Failed to load</div>
      <div className={tw`max-w-xl`}>{error.message}</div>
      <Button onPress={() => void router.invalidate()}>Retry</Button>
    </div>
  );
};
