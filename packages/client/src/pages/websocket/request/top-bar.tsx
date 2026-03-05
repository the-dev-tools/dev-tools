import { useLiveQuery } from '@tanstack/react-db';
import { Button as AriaButton, MenuTrigger } from 'react-aria-components';
import { FiMoreHorizontal } from 'react-icons/fi';
import { WebSocketCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/web_socket';
import { Button } from '@the-dev-tools/ui/button';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { useApiCollection } from '~/shared/api';
import { eqStruct, pick } from '~/shared/lib';

export interface WebSocketTopBarProps {
  websocketId: Uint8Array;
}

export const WebSocketTopBar = ({ websocketId }: WebSocketTopBarProps) => {
  const collection = useApiCollection(WebSocketCollectionSchema);

  const name =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where(eqStruct({ websocketId }))
          .select((_) => pick(_.item, 'name'))
          .findOne(),
      [collection, websocketId],
    ).data?.name ?? 'WebSocket';

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => {
      if (_ === name) return;
      void collection.utils.update({ name: _, websocketId });
    },
    value: name,
  });

  return (
    <div className='flex items-center gap-2 border-b border-neutral px-4 py-2.5'>
      <div
        className={tw`
          flex min-w-0 flex-1 gap-1 text-md leading-5 font-medium tracking-tight text-neutral-higher select-none
        `}
      >
        {isEditing ? (
          <TextInputField
            aria-label='WebSocket name'
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
      </div>

      <MenuTrigger {...menuTriggerProps}>
        <Button className={tw`p-1`} variant='ghost'>
          <FiMoreHorizontal className={tw`size-4 text-on-neutral-low`} />
        </Button>

        <Menu {...menuProps}>
          <MenuItem onAction={() => void edit()}>Rename</MenuItem>

          <MenuItem onAction={() => collection.utils.delete({ websocketId })} variant='danger'>
            Delete
          </MenuItem>
        </Menu>
      </MenuTrigger>
    </div>
  );
};
