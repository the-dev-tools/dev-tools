import { useEffect, useState } from 'react';
import { MenuTrigger } from 'react-aria-components';
import { Button } from '@the-dev-tools/ui/button';
import { Menu, MenuItem } from '@the-dev-tools/ui/menu';
import { tw } from '@the-dev-tools/ui/tailwind-literal';

type State =
  | { account: StresseurAccount; kind: 'signed-in' }
  | { kind: 'failed'; message: string }
  | { kind: 'loading' }
  | { kind: 'signed-out' }
  | { kind: 'waiting'; userCode: string };

/** Stresseur account in the header. Desktop only: the main process holds the login. */
export const AccountButton = () => {
  const api = typeof window === 'undefined' ? undefined : window.electron.account;
  const [state, setState] = useState<State>({ kind: 'loading' });

  useEffect(() => {
    if (!api) return;
    void api.get().then((account) => void setState(account ? { account, kind: 'signed-in' } : { kind: 'signed-out' }));
    return api.onEvent((event) => {
      if (event.kind === 'signed-in') setState({ account: event.account, kind: 'signed-in' });
      if (event.kind === 'failed') setState({ kind: 'failed', message: event.message });
      if (event.kind === 'waiting') setState({ kind: 'waiting', userCode: event.userCode });
    });
  }, [api]);

  if (!api || state.kind === 'loading') return null;

  if (state.kind === 'waiting')
    return (
      <div className={tw`flex items-center gap-2 font-normal`}>
        <span className={tw`text-on-inverse-low`}>Confirm in your browser:</span>
        <span className={tw`font-mono font-semibold`}>{state.userCode}</span>
        <Button
          className={tw`px-2 py-1`}
          onPress={() => {
            api.cancel();
            setState({ kind: 'signed-out' });
          }}
          variant='ghost dark'
        >
          Cancel
        </Button>
      </div>
    );

  if (state.kind === 'signed-in')
    return (
      <MenuTrigger>
        <Button className={tw`px-2 py-1`} variant='ghost dark'>
          {state.account.workspace.name}
        </Button>

        <Menu>
          <MenuItem isDisabled>{state.account.user.email}</MenuItem>
          <MenuItem onAction={() => void api.open()}>Open Stresseur Cloud</MenuItem>
          <MenuItem
            onAction={() => void api.signOut().then(() => void setState({ kind: 'signed-out' }))}
            variant='danger'
          >
            Sign out
          </MenuItem>
        </Menu>
      </MenuTrigger>
    );

  return (
    <div className={tw`flex items-center gap-2`}>
      {state.kind === 'failed' && <span className={tw`font-normal text-on-inverse-low`}>{state.message}</span>}
      <Button className={tw`px-2 py-1`} onPress={() => void api.signIn()} variant='ghost dark'>
        Sign in
      </Button>
    </div>
  );
};
