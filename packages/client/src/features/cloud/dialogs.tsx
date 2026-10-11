import { ReactNode, useRef, useState } from 'react';
import { Dialog, Heading } from 'react-aria-components';
import { FiX } from 'react-icons/fi';
import { Button } from '@the-dev-tools/ui/button';
import { Modal, useProgrammaticModal } from '@the-dev-tools/ui/modal';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField } from '@the-dev-tools/ui/text-field';
import { cloudApi } from './lib';

/** Hosts one dialog at a time. `ask` resolves with the value passed to `done`, or null when dismissed. */
export const useDialogHost = () => {
  const modal = useProgrammaticModal();
  const dismiss = useRef<(() => void) | null>(null);

  const ask = <T,>(build: (done: (value: null | T) => void) => ReactNode) =>
    new Promise<null | T>((resolve) => {
      dismiss.current?.();
      const done = (value: null | T) => {
        dismiss.current = null;
        resolve(value);
        modal.onOpenChange(false);
      };
      dismiss.current = () => void resolve(null);
      modal.onOpenChange(true, build(done));
    });

  const show = (build: (close: () => void) => ReactNode) => void ask<null>((done) => build(() => void done(null)));

  const render = modal.children && (
    <Modal
      className={tw`h-auto max-h-[80vh] w-full max-w-lg`}
      isDismissable
      isOpen={modal.isOpen}
      onOpenChange={(isOpen) => {
        if (isOpen) return;
        dismiss.current?.();
        dismiss.current = null;
        modal.onOpenChange(false);
      }}
    >
      {modal.children}
    </Modal>
  );

  return { ask, render, show };
};

interface CloudDialogProps {
  actions?: ReactNode;
  children?: ReactNode;
  onClose: () => void;
  title: string;
}

export const CloudDialog = ({ actions, children, onClose, title }: CloudDialogProps) => (
  <Dialog className={tw`flex max-h-[80vh] flex-col outline-hidden`}>
    <div className={tw`flex items-center justify-between px-5 pt-4`}>
      <Heading className={tw`text-lg/6 font-semibold tracking-tight text-on-neutral`} slot='title'>
        {title}
      </Heading>

      <Button aria-label='Close' className={tw`p-1`} onPress={onClose} variant='ghost'>
        <FiX className={tw`size-4 text-on-neutral-low`} />
      </Button>
    </div>

    <div className={tw`flex min-h-0 flex-1 flex-col gap-3 overflow-auto px-5 py-3 text-md/5 text-on-neutral`}>
      {children}
    </div>

    {actions && <div className={tw`flex justify-end gap-2 border-t border-neutral px-5 py-3`}>{actions}</div>}
  </Dialog>
);

export type KeepOrTake = 'main' | 'mine';

export const KeepOrTakeDialog = ({ done }: { done: (value: KeepOrTake | null) => void }) => (
  <CloudDialog
    actions={
      <>
        <Button onPress={() => void done('mine')}>Keep mine</Button>
        <Button onPress={() => void done('main')} variant='primary'>
          Take main
        </Button>
      </>
    }
    onClose={() => void done(null)}
    title='You have unsaved edits'
  >
    Main has changed since you opened this flow.
  </CloudDialog>
);

interface EnvPromptProps {
  done: (value: null | Record<string, string>) => void;
  initial: Record<string, string>;
  names: string[];
}

export const EnvPrompt = ({ done, initial, names }: EnvPromptProps) => {
  const [values, setValues] = useState(initial);

  return (
    <CloudDialog
      actions={
        <>
          <Button onPress={() => void done(null)}>Cancel</Button>
          <Button onPress={() => void done(values)} variant='primary'>
            Run
          </Button>
        </>
      }
      onClose={() => void done(null)}
      title='Variables for this run'
    >
      <p className={tw`text-on-neutral-low`}>Not in your clone&apos;s .env. Leave empty to skip.</p>

      {names.map((name) => (
        <TextInputField
          key={name}
          label={name}
          onChange={(value) => void setValues((_) => ({ ..._, [name]: value }))}
          value={values[name] ?? ''}
        />
      ))}
    </CloudDialog>
  );
};

interface CloudErrorViewProps {
  apiUrl?: string | undefined;
  error: StresseurCloudError;
}

/** The server's message plus its fix link; "Upgrade" only when the server says so. */
export const CloudErrorView = ({ apiUrl, error }: CloudErrorViewProps) => {
  const cloud = cloudApi();

  if (error.kind !== 'error') {
    const text = { offline: 'Offline', 'sign-in-again': 'Sign in again', 'signed-out': 'Sign in first' }[error.kind];
    return <p className={tw`text-danger`}>{text}</p>;
  }

  const upgradeUrl = error.fixUrl ?? apiUrl;

  return (
    <div className={tw`flex flex-col items-start gap-2`}>
      <p className={tw`text-danger`}>{error.message}</p>

      <div className={tw`flex gap-2`}>
        {error.upgrade && upgradeUrl && (
          <Button onPress={() => void cloud?.open(upgradeUrl)} variant='primary'>
            Upgrade
          </Button>
        )}

        {error.fixUrl && !error.upgrade && (
          <Button onPress={() => void (error.fixUrl && cloud?.open(error.fixUrl))}>Fix in Stresseur Cloud</Button>
        )}
      </div>
    </div>
  );
};
