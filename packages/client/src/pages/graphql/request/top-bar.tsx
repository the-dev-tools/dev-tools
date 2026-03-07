import { Array, pipe } from 'effect';
import { useTransition } from 'react';
import { Button as AriaButton, DialogTrigger, MenuTrigger } from 'react-aria-components';
import { FiClock, FiMoreHorizontal } from 'react-icons/fi';
import { GraphQLService } from '@the-dev-tools/spec/buf/api/graph_q_l/v1/graph_q_l_pb';
import {
  GraphQLCollectionSchema,
  GraphQLDeltaCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/graph_q_l';
import { Button } from '@the-dev-tools/ui/button';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { DeltaResetButton, useDeltaState } from '~/features/delta';
import { request, useApiCollection } from '~/shared/api';
import { routes } from '~/shared/routes';
import { HistoryModal } from '../history';
import { GraphQLUrl } from './url';

export interface GraphQLTopBarProps {
  deltaGraphqlId?: Uint8Array | undefined;
  graphqlId: Uint8Array;
}

export const GraphQLTopBar = ({ deltaGraphqlId, graphqlId }: GraphQLTopBarProps) => {
  const { transport } = routes.root.useRouteContext();

  const collection = useApiCollection(GraphQLCollectionSchema);
  const deltaCollection = useApiCollection(GraphQLDeltaCollectionSchema);

  const deltaOptions = {
    deltaId: deltaGraphqlId,
    deltaSchema: GraphQLDeltaCollectionSchema,
    isDelta: deltaGraphqlId !== undefined,
    originId: graphqlId,
    originSchema: GraphQLCollectionSchema,
  };

  const [name, setName] = useDeltaState({ ...deltaOptions, valueKey: 'name' });

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => {
      if (_ === name) return;
      setName(_);
    },
    value: name ?? '',
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
              {name}
            </AriaButton>
          )}

          <DeltaResetButton {...deltaOptions} valueKey='name' />
        </div>

        <DialogTrigger>
          <Button className={tw`px-2 py-1 text-on-neutral`} variant='ghost'>
            <FiClock className={tw`size-4 text-on-neutral-low`} /> Response History
          </Button>

          <HistoryModal deltaGraphqlId={deltaGraphqlId} graphqlId={graphqlId} />
        </DialogTrigger>

        <MenuTrigger {...menuTriggerProps}>
          <Button className={tw`p-1`} variant='ghost'>
            <FiMoreHorizontal className={tw`size-4 text-on-neutral-low`} />
          </Button>

          <Menu {...menuProps}>
            <MenuItem onAction={() => void edit()}>Rename</MenuItem>

            <MenuItem
              onAction={() => {
                if (deltaGraphqlId) deltaCollection.utils.delete({ deltaGraphqlId });
                else collection.utils.delete({ graphqlId });
              }}
              variant='danger'
            >
              Delete
            </MenuItem>
          </Menu>
        </MenuTrigger>
      </div>

      <div className={tw`flex gap-3 p-6 pb-0`}>
        <GraphQLUrl deltaGraphqlId={deltaGraphqlId} graphqlId={graphqlId} />

        <Button
          className={tw`px-6`}
          isPending={isSending}
          onPress={() =>
            void startTransition(async () => {
              const graphqlTransactions = Array.fromIterable(collection._state.transactions.values());
              const deltaTransactions = Array.fromIterable(deltaCollection._state.transactions.values());

              await pipe(
                Array.appendAll(graphqlTransactions, deltaTransactions),
                Array.map((_) => _.isPersisted.promise),
                (_) => Promise.all(_),
              );

              await request({
                input: { graphqlId: deltaGraphqlId ?? graphqlId },
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
