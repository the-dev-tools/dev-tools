import { getRouteApi } from '@tanstack/react-router';

export const rootRouteApi = getRouteApi('__root__');
export const welcomeRouteApi = getRouteApi('/');
export const workspaceRouteApi = getRouteApi('/workspace/$workspaceIdCan');
export const overviewRouteApi = getRouteApi('/workspace/$workspaceIdCan/');
export const httpRouteApi = getRouteApi('/workspace/$workspaceIdCan/http/$httpIdCan');
export const flowLayoutRouteApi = getRouteApi('/workspace/$workspaceIdCan/flow/$flowIdCan');
export const flowEditRouteApi = getRouteApi('/workspace/$workspaceIdCan/flow/$flowIdCan/');
export const flowHistoryRouteApi = getRouteApi('/workspace/$workspaceIdCan/flow/$flowIdCan/history');
