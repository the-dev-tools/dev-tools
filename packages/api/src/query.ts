import { create, DescMessage, DescMethodUnary, Message, MessageInitShape, MessageShape } from '@bufbuild/protobuf';
import { GenMessage } from '@bufbuild/protobuf/codegenv1';
import {
  createConnectQueryKey,
  createProtobufSafeUpdater,
  useMutation as useConnectMutation,
  UseMutationOptions,
  useTransport,
} from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';

export const useCreateMutation = <
  BaseData extends Record<string, unknown>,
  ListData extends BaseData,
  CreateInput extends GenMessage<Message & BaseData>,
  CreateOutput extends GenMessage<Message & ListData>,
  ListInput extends DescMessage,
  ListOutput extends GenMessage<Message & { items: ListData[] }>,
  Context = unknown,
>(
  createMutation: DescMethodUnary<CreateInput, CreateOutput>,
  updateOptions: {
    key: Exclude<keyof MessageShape<CreateOutput>, keyof Message>;
    listQuery: DescMethodUnary<ListInput, ListOutput>;
    listInput?: MessageInitShape<ListInput>;
  },
  mutationOptions?: UseMutationOptions<CreateInput, CreateOutput, Context>,
) => {
  const queryClient = useQueryClient();
  const transport = useTransport();

  return useConnectMutation(createMutation, {
    transport,
    ...mutationOptions,
    onSuccess: (output, input, context) => {
      mutationOptions?.onSuccess?.(output, input, context);

      const queryKey = createConnectQueryKey({
        schema: updateOptions.listQuery,
        input: updateOptions.listInput ?? {},
        cardinality: 'finite',
        transport,
      });

      const listItem = create(updateOptions.listQuery.output.field.items.message!, {
        ...input,
        [updateOptions.key]: output[updateOptions.key],
      });

      const updater = createProtobufSafeUpdater(updateOptions.listQuery, (old) =>
        create(updateOptions.listQuery.output, {
          items: [...(old?.items ?? []), listItem],
        } as MessageInitShape<ListOutput>),
      );

      queryClient.setQueryData(queryKey, updater);
    },
  });
};
