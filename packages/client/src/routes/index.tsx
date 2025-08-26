import { getRouteApi } from '@tanstack/react-router';

export const rootRouteApi = getRouteApi('__root__');
export const welcomeRouteApi = getRouteApi('/');
export const workspaceRouteApi = getRouteApi('/workspace/$workspaceIdCan');
export const overviewRouteApi = getRouteApi('/workspace/$workspaceIdCan/');
export const requestRouteApi = getRouteApi('/workspace/$workspaceIdCan/request/$endpointIdCan/$exampleIdCan');
export const flowLayoutRouteApi = getRouteApi('/workspace/$workspaceIdCan/flow/$flowIdCan');
export const flowEditRouteApi = getRouteApi('/workspace/$workspaceIdCan/flow/$flowIdCan/');
export const flowHistoryRouteApi = getRouteApi('/workspace/$workspaceIdCan/flow/$flowIdCan/history');
