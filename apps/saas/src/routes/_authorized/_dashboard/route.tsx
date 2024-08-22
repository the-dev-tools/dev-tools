import { createFileRoute } from '@tanstack/react-router';

import { DashboardLayout } from '../../../dashboard';

export const Route = createFileRoute('/_authorized/_dashboard')({
  component: DashboardLayout,
});
