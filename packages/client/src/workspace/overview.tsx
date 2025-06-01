import { createFileRoute } from '@tanstack/react-router';

const makeRoute = createFileRoute('/_authorized/workspace/$workspaceIdCan/');

// TODO: implement overview
export const Route = makeRoute();
