import { createFileRoute } from '@tanstack/react-router';
import { Background, BackgroundVariant, ReactFlow } from '@xyflow/react';

import { tw } from '@the-dev-tools/ui/tailwind-literal';

export const Route = createFileRoute('/_authorized/workspace/$workspaceIdCan/flow/$flowIdCan')({
  component: RouteComponent,
});

function RouteComponent() {
  return (
    <ReactFlow proOptions={{ hideAttribution: true }} colorMode='light'>
      <Background
        variant={BackgroundVariant.Dots}
        size={2}
        gap={20}
        color='currentColor'
        className={tw`text-slate-300`}
      />
    </ReactFlow>
  );
}
