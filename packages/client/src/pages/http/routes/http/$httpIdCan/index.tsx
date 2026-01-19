import { createFileRoute } from '@tanstack/react-router';
import { openTab } from '~/widgets/tabs';
import { HttpPage } from '../../../page';
import { HttpTab, httpTabId } from '../../../tab';

export const Route = createFileRoute('/(dashboard)/(workspace)/workspace/$workspaceIdCan/(http)/http/$httpIdCan/')({
  component: HttpPage,
  onEnter: async (match) => {
    const { httpId } = match.context;

    await openTab({
      id: httpTabId({ httpId }),
      match,
      node: <HttpTab httpId={httpId} />,
    });
  },
});
