import { Array, pipe } from 'effect';
import { useTransition } from 'react';
import { Button as AriaButton, MenuTrigger } from 'react-aria-components';
import { FiMoreHorizontal } from 'react-icons/fi';
import { GraphQLService } from '@the-dev-tools/spec/buf/api/graph_q_l/v1/graph_q_l_pb';
import { GraphQLCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/graph_q_l';
import { Button } from '@the-dev-tools/ui/button';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { request, useApiCollection } from '~/shared/api';
import { routes } from '~/shared/routes';

export interface GraphQLTopBarProps {
  graphqlId: Uint8Array;
}

export const GraphQLTopBar = ({ graphqlId }: GraphQLTopBarProps) => {
  const { transport } = routes.root.useRouteContext();

  const collection = useApiCollection(GraphQLCollectionSchema);

  const item = collection.get(collection.utils.getKey({ graphqlId }));

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => {
      if (_ === item?.name) return;
      collection.utils.update({ graphqlId, name: _ });
    },
    value: item?.name ?? '',
  });

  const [isSending, startTransition] = useTransition();

  return (
    <>
      <div className='flex items-center gap-2 border-b border-neutral px-4 py-2.5'>
        <div
          className={tw`
            flex min-w-0 flex-1 gap-1 text-md leading-5 font-medium tracking-tight text-neutral-higher select-none
          `}
        >
          {isEditing ? (
            <TextInputField
              aria-label='GraphQL request name'
              inputClassName={tw`-my-1 py-1 leading-none text-on-neutral`}
              {...textFieldProps}
            />
          ) : (
            <AriaButton
              className={tw`max-w-full cursor-text truncate text-on-neutral`}
              onContextMenu={onContextMenu}
              onPress={() => void edit()}
            >
              {item?.name}
            </AriaButton>
          )}
        </div>

        <MenuTrigger {...menuTriggerProps}>
          <Button className={tw`p-1`} variant='ghost'>
            <FiMoreHorizontal className={tw`size-4 text-on-neutral-low`} />
          </Button>

          <Menu {...menuProps}>
            <MenuItem onAction={() => void edit()}>Rename</MenuItem>

            <MenuItem onAction={() => collection.utils.delete({ graphqlId })} variant='danger'>
              Delete
            </MenuItem>
          </Menu>
        </MenuTrigger>
      </div>

      <div className={tw`flex gap-3 p-6 pb-0`}>
        <TextInputField
          aria-label='URL'
          className={tw`flex-1`}
          inputClassName={tw`font-mono text-sm`}
          onChange={(_) => collection.utils.update({ graphqlId, url: _ })}
          placeholder='Enter GraphQL endpoint URL'
          value={item?.url ?? ''}
        />

        <Button
          className={tw`px-6`}
          isPending={isSending}
          onPress={() =>
            void startTransition(async () => {
              const transactions = Array.fromIterable(collection._state.transactions.values());

              await pipe(
                transactions,
                Array.map((_) => _.isPersisted.promise),
                (_) => Promise.all(_),
              );

              await request({
                input: { graphqlId },
                method: GraphQLService.method.graphQLRun,
                transport,
              });
            })
          }
          variant='primary'
        >
          Send
        </Button>
      </div>
    </>
  );
};
