import { fromJson, JsonValue, toJson } from '@bufbuild/protobuf';
import { ValueSchema } from '@bufbuild/protobuf/wkt';
import { Code, ConnectError, ConnectRouter } from '@connectrpc/connect';
import { SourceTextModule } from 'node:vm';

import { NodeJSExecutorService as NodeJSExecutorServiceSchema } from '@the-dev-tools/spec/nodejs_executor/v1/nodejs_executor_pb';

export const NodeJSExecutorService = (router: ConnectRouter) =>
  router.service(NodeJSExecutorServiceSchema, {
    executeNodeJS: async (request) => {
      const module = new SourceTextModule(request.code);

      await module.link(() => {
        throw new ConnectError('Importing dependencies is not supported', Code.Unimplemented);
      });

      await module.evaluate();

      if (!('default' in module.namespace)) {
        // ? Can be implemented in the future via CDN imports
        // https://dev.to/mxfellner/dynamic-import-with-http-urls-in-node-js-7og
        throw new ConnectError('Default export must be present', Code.InvalidArgument);
      }

      let result = module.namespace.default;

      if (typeof result === 'function') {
        const context = request.context ? toJson(ValueSchema, request.context) : {};
        // eslint-disable-next-line @typescript-eslint/no-unsafe-call
        result = result(context);
      }

      return { result: fromJson(ValueSchema, result as JsonValue) };
    },
  });
