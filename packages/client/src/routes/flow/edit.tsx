import { createFileRoute } from '@tanstack/react-router';
import { Spinner } from '@the-dev-tools/ui/spinner';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { FlowEditPage } from '~flow/flow';

export const Route = createFileRoute('/workspace/$workspaceIdCan/flow/$flowIdCan/')({
  component: FlowEditPage,
  pendingComponent: () => (
    <div className={tw`flex h-full items-center justify-center`}>
      <Spinner size='xl' />
    </div>
  ),
});
