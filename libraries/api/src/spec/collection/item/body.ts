import {
  bodyRawGet,
  bodyRawUpdate,
} from '@the-dev-tools/spec/collection/item/body/v1/body-RequestService_connectquery';

import { MutationSpec } from '../../../query.internal';

export const bodyRawUpdateSpec = {
  mutation: bodyRawUpdate,
  key: 'exampleId',
  parentKeys: ['workspaceId', 'collectionId', 'endpointId'],
  onSuccess: [['query - get - update cache', { query: bodyRawGet }]],
} satisfies MutationSpec;
