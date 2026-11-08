import { Tooltip, TooltipTrigger } from 'react-aria-components';
import { FiCircle, FiX } from 'react-icons/fi';
import { twJoin } from 'tailwind-merge';
import { Button } from '@the-dev-tools/ui/button';
import { Spinner } from '@the-dev-tools/ui/spinner';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { openLaunchpad } from './launchpad';
import { failedStep, type InitRun, setRun, stopRecording, useInitRun } from './launchpad-run';
import { recordApi } from './lib';

const failedLabel = {
  detected: 'Could not start',
  generated: 'Generate failed',
  recording: 'Recording failed',
  verified: 'Checks failed',
};

const busyLabel = (run: InitRun) => {
  if (run.status === 'importing') return 'Opening in Studio';
  if (run.generated) return 'Running checks';
  if (run.recorded) return 'Generating flows';
  return run.startingApp && !run.detected ? 'Starting app' : 'Starting';
};

/** Header: Record opens "Record it myself" (last URL prefilled); while a run goes, its state. */
export const RecordControl = () => {
  const run = useInitRun();

  if (!recordApi()) return null;

  const text = tw`flex items-center gap-2 text-xs/4 font-normal tracking-tight text-on-inverse`;
  const open = () => void openLaunchpad('run');

  if (run?.status === 'running' && run.recording && !run.recorded)
    return (
      <div className={text}>
        <Button className={tw`gap-2 px-1.5 py-0.5 text-xs/4`} onPress={open} variant='ghost dark'>
          <FiCircle className={tw`size-2.5 animate-pulse fill-danger stroke-danger`} />
          {run.calls === 1 ? '1 call' : `${String(run.calls)} calls`}
        </Button>
        <Button className={tw`px-1.5 py-0.5 text-xs/4`} onPress={() => void stopRecording()} variant='ghost dark'>
          Done
        </Button>
      </div>
    );

  if (run && run.status !== 'finished')
    return (
      <Button className={twJoin(text, tw`px-1.5 py-0.5`)} onPress={open} variant='ghost dark'>
        <Spinner />
        {busyLabel(run)}
      </Button>
    );

  const failed = run && failedStep(run);

  return (
    <div className={text}>
      {run && failed && (
        <>
          <Button className={tw`px-1.5 py-0.5 text-xs/4 text-danger`} onPress={open} variant='ghost dark'>
            {failedLabel[failed.key]}
          </Button>
          <Button
            aria-label='Dismiss'
            className={tw`p-0.5`}
            onPress={() => void setRun(undefined)}
            variant='ghost dark'
          >
            <FiX className={tw`size-3`} />
          </Button>
        </>
      )}

      <TooltipTrigger delay={750}>
        <Button
          className={tw`gap-1.5 px-2 py-1 text-xs/4 font-semibold`}
          onPress={() => {
            if (run) setRun(undefined);
            openLaunchpad('record');
          }}
          variant='ghost dark'
        >
          <FiCircle className={tw`size-2.5 fill-danger stroke-danger`} />
          Record
        </Button>
        <Tooltip className={tw`rounded-md bg-inverse px-2 py-1 text-xs text-on-inverse`}>
          Record your app and get flows
        </Tooltip>
      </TooltipTrigger>
    </div>
  );
};
