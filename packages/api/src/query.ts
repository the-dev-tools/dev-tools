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

type ListItem<ListOutput> =
  ListOutput extends GenMessage<
    Message & {
      items: (infer Item)[];
    }
  >
    ? Item
    : never;

export const useCreateMutation = <
  CreateInput extends DescMessage,
  CreateOutput extends DescMessage,
  ListInput extends DescMessage,
  ListOutput extends GenMessage<Message & { items: unknown[] }>,
  Context = unknown,
>(
  createMutation: DescMethodUnary<CreateInput, CreateOutput>,
  updateOptions: {
    listQuery: DescMethodUnary<ListInput, ListOutput>;
    listInput?: MessageInitShape<ListInput>;
    toListItem?: (
      input: Omit<MessageInitShape<CreateInput>, keyof Message>,
      output: Omit<MessageShape<CreateOutput>, keyof Message>,
    ) => MessageInitShape<GenMessage<Message & ListItem<ListOutput>>>;
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

      const listItem = create(
        updateOptions.listQuery.output.field.items.message!,
        updateOptions.toListItem?.(input, output) ?? { ...input, ...output },
      );

      const updater = createProtobufSafeUpdater(updateOptions.listQuery, (old) =>
        create(updateOptions.listQuery.output, {
          items: [...(old?.items ?? []), listItem],
        } as MessageInitShape<ListOutput>),
      );

      queryClient.setQueryData(queryKey, updater);
    },
  });
};
