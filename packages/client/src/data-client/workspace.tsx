import { MessageInitShape } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { Endpoint, EntityMixin, schema } from '@data-client/endpoint';
import { Ulid } from 'id128';

import {
  WorkspaceCreateRequestSchema,
  WorkspaceDeleteRequestSchema,
  WorkspaceSchema,
  WorkspaceService,
  WorkspaceUpdateRequestSchema,
} from '@the-dev-tools/spec/workspace/v1/workspace_pb';

import { methodName, toClass, transport } from './util';

export class Workspace extends toClass(WorkspaceSchema) {}

export class WorkspaceEntity extends EntityMixin(Workspace, {
  key: WorkspaceSchema.typeName,
  pk: (_) => Ulid.construct(_.workspaceId).toCanonical(),
}) {}

const workspaceClient = createClient(WorkspaceService, transport);

export const workspaceList = new Endpoint(workspaceClient.workspaceList, {
  name: methodName(WorkspaceService, 'workspaceList'),
  schema: { items: new schema.Collection([WorkspaceEntity]) },
});

export const workspaceCreate = new Endpoint(
  async (request: MessageInitShape<typeof WorkspaceCreateRequestSchema>) => {
    const response = await workspaceClient.workspaceCreate(request);
    return { ...request, ...response };
  },
  {
    name: methodName(WorkspaceService, 'workspaceCreate'),
    schema: new schema.Collection([WorkspaceEntity]).push,
    sideEffect: true,
  },
);

export const workspaceUpdate = new Endpoint(
  async (request: MessageInitShape<typeof WorkspaceUpdateRequestSchema>) => {
    await workspaceClient.workspaceUpdate(request);
    return request;
  },
  {
    name: methodName(WorkspaceService, 'workspaceUpdate'),
    schema: WorkspaceEntity,
    sideEffect: true,
  },
);

export const workspaceDelete = new Endpoint(
  async (request: MessageInitShape<typeof WorkspaceDeleteRequestSchema>) => {
    await workspaceClient.workspaceDelete(request);
    return request;
  },
  {
    name: methodName(WorkspaceService, 'workspaceDelete'),
    schema: new schema.Invalidate(WorkspaceEntity),
    sideEffect: true,
  },
);
