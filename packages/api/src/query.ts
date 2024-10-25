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

type ListItem<ListResponse> =
  ListResponse extends GenMessage<
    Message & {
      items: (infer Item)[];
    }
  >
    ? Item
    : never;

export const useCreateMutation = <
  CreateRequest extends DescMessage,
  CreateResponse extends DescMessage,
  ListRequest extends DescMessage,
  ListResponse extends GenMessage<Message & { items: unknown[] }>,
  Context = unknown,
>(
  createMutation: DescMethodUnary<CreateRequest, CreateResponse>,
  updateOptions: {
    listQuery: DescMethodUnary<ListRequest, ListResponse>;
    toListInput?: (
      input: Omit<MessageInitShape<CreateRequest>, keyof Message>,
      output: Omit<MessageShape<CreateResponse>, keyof Message>,
    ) => MessageInitShape<ListRequest>;
    toListItem?: (
      input: Omit<MessageInitShape<CreateRequest>, keyof Message>,
      output: Omit<MessageShape<CreateResponse>, keyof Message>,
    ) => MessageInitShape<GenMessage<Message & ListItem<ListResponse>>>;
  },
  mutationOptions?: UseMutationOptions<CreateRequest, CreateResponse, Context>,
) => {
  const queryClient = useQueryClient();
  const transport = useTransport();

  return useConnectMutation(createMutation, {
    transport,
    ...mutationOptions,
    onSuccess: (output, input, context) => {
      mutationOptions?.onSuccess?.(output, input, context);

      const createItem = { ...input, ...output };

      const queryKey = createConnectQueryKey({
        schema: updateOptions.listQuery,
        input: updateOptions.toListInput?.(input, output) ?? createItem,
        cardinality: 'finite',
        transport,
      });

      const listItem = create(
        updateOptions.listQuery.output.field.items.message!,
        updateOptions.toListItem?.(input, output) ?? createItem,
      );

      const updater = createProtobufSafeUpdater(updateOptions.listQuery, (old) =>
        create(updateOptions.listQuery.output, {
          items: [...(old?.items ?? []), listItem],
        } as MessageInitShape<ListResponse>),
      );

      queryClient.setQueryData(queryKey, updater);
    },
  });
};
