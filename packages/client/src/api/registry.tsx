import { createRegistry } from '@bufbuild/protobuf';

import { files } from '@the-dev-tools/spec/files';

export const registry = createRegistry(...files);
