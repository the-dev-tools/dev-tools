import { DescMessage, DescMethodUnary, MessageInitShape, MessageShape } from '@bufbuild/protobuf';
import { Transport } from '@connectrpc/connect';
import { QueryClient } from '@tanstack/react-query';

export interface SpecFnParams {
  query?: DescMethodUnary;
}

export interface MutationSpec<Input extends DescMessage = DescMessage, Output extends DescMessage = DescMessage> {
  mutation: DescMethodUnary<Input, Output>;
  key?: string;
  parentKeys?: string[];
  onSuccess?: [string, SpecFnParams][];
}

export interface SpecFnArgs {
  input: MessageInitShape<DescMessage>;
  output: MessageShape<DescMessage>;
  params: SpecFnParams;
  queryClient: QueryClient;
  spec: MutationSpec;
  transport: Transport;
}

export type SpecFn = (args: SpecFnArgs) => void;
