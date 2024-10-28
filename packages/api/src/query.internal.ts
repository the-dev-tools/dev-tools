import { DescMessage, DescMethodUnary, Message, MessageInitShape, MessageShape } from '@bufbuild/protobuf';
import { Transport } from '@connectrpc/connect';
import { QueryClient } from '@tanstack/react-query';

export interface SpecFnParams {
  query?: DescMethodUnary;
  queryInputFn?: string;
  compareItemFn?: string;
  createItemFn?: string;
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

export type SpecFn<T> = (args: SpecFnArgs) => T;
export type SpecOnSuccessFn = SpecFn<void>;
export type SpecQueryInputFn = SpecFn<MessageInitShape<DescMessage>>;
export type SpecCompareItemFn = SpecFn<
  (a: MessageInitShape<DescMessage>) => (b: MessageInitShape<DescMessage>) => boolean
>;
export type SpecCreateItemFn = SpecFn<(old?: MessageShape<DescMessage>) => Message>;
