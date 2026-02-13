import { Atom, Result, useAtomValue } from '@effect-atom/atom-react';
import { HttpClient, HttpClientResponse } from '@effect/platform';
import { Cause, Effect, Layer, pipe, Schema } from 'effect';
import { useEffect, useState } from 'react';
import { createRoot } from 'react-dom/client';
import Markdown from 'react-markdown';
import { addGlobalLayer, App as Client, configProviderFromMetaEnv, runtimeAtom } from '@the-dev-tools/client';
import { Button } from '@the-dev-tools/ui/button';
import { Logo } from '@the-dev-tools/ui/illustrations';
import { ProgressBar } from '@the-dev-tools/ui/progress-bar';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import packageJson from '../../package.json';

import './styles.css';

pipe(configProviderFromMetaEnv({ VERSION: packageJson.version }), Layer.setConfigProvider, addGlobalLayer);

const updateCheckAtom = runtimeAtom.atom(
  Effect.gen(function* () {
    const client = pipe(
      yield* HttpClient.HttpClient,
      HttpClient.followRedirects(3),
      HttpClient.withTracerPropagation(false),
    );

    const version = yield* Effect.tryPromise(() => window.electron.update.check());

    if (!version) return yield* new Cause.NoSuchElementException();

    const { body } = yield* pipe(
      client.get(`https://api.github.com/repos/the-dev-tools/dev-tools/releases/tags/desktop@${version}`),
      Effect.flatMap(
        HttpClientResponse.schemaBodyJson(
          Schema.Struct({
            body: Schema.String,
          }),
        ),
      ),
    );

    return body;
  }),
);

interface UpdateAvailableProps {
  children: string;
}

const UpdateAvailable = ({ children }: UpdateAvailableProps) => {
  const [state, setState] = useState<'init' | 'skip' | 'update'>('init');

  if (state === 'skip') return <Client />;

  return (
    <div className={tw`flex h-full flex-col items-center gap-8 p-16`}>
      <div className={tw`text-center`}>
        <div className={tw`flex items-center gap-4 text-4xl font-semibold`}>
          <Logo className={tw`size-10`} />
          DevTools Studio
        </div>

        <div className={tw`mt-2 text-2xl`}>Update available!</div>
      </div>

      {/* eslint-disable-next-line better-tailwindcss/no-unregistered-classes */}
      <div className={tw`prose dark:prose-invert flex-1 overflow-auto`}>
        <Markdown>{children}</Markdown>
      </div>

      {state === 'init' && (
        <div className={tw`flex gap-4`}>
          <Button
            onPress={() => {
              window.electron.update.start();
              setState('update');
            }}
            variant='primary'
          >
            Update
          </Button>

          <Button onPress={() => void setState('skip')}>Skip</Button>
        </div>
      )}

      {state === 'update' && <UpdateProgress />}
    </div>
  );
};

const UpdateProgress = () => {
  const [percent, setPercent] = useState(0);

  useEffect(() => {
    window.electron.update.onProgress((_) => {
      setPercent(_.percent);
    });
  }, []);

  return <ProgressBar label='Updating...' value={percent} />;
};

const finalizerAtom = Atom.make((_) => void _.addFinalizer(() => void window.electron.onCloseDone()));

const App = () => {
  useAtomValue(finalizerAtom);

  const updateCheck = useAtomValue(updateCheckAtom);

  return Result.match(updateCheck, {
    onFailure: () => <Client />,
    onInitial: () => 'Loading...',
    onSuccess: (_) => <UpdateAvailable>{_.value}</UpdateAvailable>,
  });
};

const root = createRoot(document.getElementById('root')!);
window.electron.onClose(() => void root.unmount());
root.render(<App />);
