import { createFileRoute } from '@tanstack/react-router';
import { HttpPage, HttpTab, httpTabId } from '~/features/http';
import { openTab } from '~/tabs';

export const Route = createFileRoute('/workspace/$workspaceIdCan/http/$httpIdCan/item')({
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
