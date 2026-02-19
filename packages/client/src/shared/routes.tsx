/* eslint-disable perfectionist/sort-objects */
import { getRouteApi } from '@tanstack/react-router';

export const routes = {
  root: getRouteApi('__root__'),
  dashboard: {
    index: getRouteApi('/(dashboard)/'),
    workspace: {
      route: getRouteApi('/(dashboard)/(workspace)/workspace/$workspaceIdCan'),
      index: getRouteApi('/(dashboard)/(workspace)/workspace/$workspaceIdCan/'),
      user: {
        signIn: getRouteApi('/(dashboard)/(user)/signIn'),
        signUp: getRouteApi('/(dashboard)/(user)/signUp'),
      },
      credential: getRouteApi(
        '/(dashboard)/(workspace)/workspace/$workspaceIdCan/(credential)/credential/$credentialIdCan/',
      ),
      flow: {
        route: getRouteApi('/(dashboard)/(workspace)/workspace/$workspaceIdCan/(flow)/flow/$flowIdCan'),
        index: getRouteApi('/(dashboard)/(workspace)/workspace/$workspaceIdCan/(flow)/flow/$flowIdCan/'),
        history: getRouteApi('/(dashboard)/(workspace)/workspace/$workspaceIdCan/(flow)/flow/$flowIdCan/history'),
      },
      http: {
        route: getRouteApi('/(dashboard)/(workspace)/workspace/$workspaceIdCan/(http)/http/$httpIdCan'),
        index: getRouteApi('/(dashboard)/(workspace)/workspace/$workspaceIdCan/(http)/http/$httpIdCan/'),
        delta: getRouteApi(
          '/(dashboard)/(workspace)/workspace/$workspaceIdCan/(http)/http/$httpIdCan/delta/$deltaHttpIdCan',
        ),
      },
    },
  },
};
