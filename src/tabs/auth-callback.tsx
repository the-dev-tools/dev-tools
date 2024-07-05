import { Effect } from 'effect';
import * as React from 'react';

import * as Auth from '@/auth';
import { Runtime } from '@/runtime';

const AuthCallbackPage = () => {
  const token = new URLSearchParams(window.location.search).get('magic_credential');
  const [state, setState] = React.useState<'loading' | 'success' | 'failure'>('loading');

  React.useEffect(() => {
    if (!token) return;
    void Effect.gen(function* () {
      const success = yield* Auth.loginConfirm(token);
      setState(success ? 'success' : 'failure');
    }).pipe(Runtime.runPromise);
  }, [token]);

  if (!token) return 'Invalid token';
  if (state === 'loading') return 'Authenticating...';
  if (state === 'failure') return 'Authentication failed!';
  return 'Authentication complete! You can return to the extension.';
};

export default AuthCallbackPage;
