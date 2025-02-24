import { createFileRoute } from '@tanstack/react-router';
import { Panel } from 'react-resizable-panels';

export const Route = createFileRoute('/_authorized/workspace/$workspaceIdCan/')({
  component: Overview,
});

// TODO: implement overview
function Overview() {
  return <Panel id='main' order={2} />;
}
