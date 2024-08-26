import { createFileRoute } from '@tanstack/react-router';

import { DashboardRootLayout } from '../../../dashboard';

export const Route = createFileRoute('/_authorized/_dashboard')({
  component: DashboardRootLayout,
});
