import { fromJson, type JsonValue, toJson } from '@bufbuild/protobuf';
import { ValueSchema } from '@bufbuild/protobuf/wkt';
import { Code, ConnectError, type ConnectRouter } from '@connectrpc/connect';
import { SourceTextModule } from 'node:vm';
import { NodeJsExecutorService as NodeJsExecutorServiceSchema } from '@the-dev-tools/spec/buf/api/node_js_executor/v1/node_js_executor_pb';

export const NodeJsExecutorService = (router: ConnectRouter) =>
  router.service(NodeJsExecutorServiceSchema, {
    nodeJsExecutorRun: async (request) => {
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

      result = await Promise.resolve(result);

      return { result: fromJson(ValueSchema, result as JsonValue) };
    },
  });
