import { createFileRoute } from '@tanstack/react-router';
import { Panel } from 'react-resizable-panels';

const makeRoute = createFileRoute('/_authorized/workspace/$workspaceIdCan/');

export const Route = makeRoute({ component: Overview });

// TODO: implement overview
function Overview() {
  return <Panel id='main' order={2} />;
}
