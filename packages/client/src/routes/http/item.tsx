import { createFileRoute } from '@tanstack/react-router';
import { addTab } from '@the-dev-tools/ui/router';
import { HttpPage, HttpTab, httpTabId } from '~/features/http';

export const Route = createFileRoute('/workspace/$workspaceIdCan/http/$httpIdCan/item')({
  component: HttpPage,
  onEnter: (match) => {
    const { httpId } = match.context;

    addTab({
      id: httpTabId({ httpId }),
      match,
      node: <HttpTab httpId={httpId} />,
    });
  },
});
