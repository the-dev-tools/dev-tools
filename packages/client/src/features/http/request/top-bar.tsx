import { eq, useLiveQuery } from '@tanstack/react-db';
import { Array, Option, pipe } from 'effect';
import { useTransition } from 'react';
import { Button as AriaButton, DialogTrigger, MenuTrigger } from 'react-aria-components';
import { FiClock, FiMoreHorizontal } from 'react-icons/fi';
import { HttpService } from '@the-dev-tools/spec/api/http/v1/http_pb';
import { HttpCollectionSchema, HttpSearchParamCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { Button } from '@the-dev-tools/ui/button';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { Connect, useApiCollection } from '~/api-new';
import { rootRouteApi } from '~/routes';
import { pick } from '~/utils/tanstack-db';
import { HistoryModal } from '../history';
import { HttpUrl } from './url';

export interface HttpTopBarProps {
  httpId: Uint8Array;
}

export const HttpTopBar = ({ httpId }: HttpTopBarProps) => {
  const { transport } = rootRouteApi.useRouteContext();

  const httpCollection = useApiCollection(HttpCollectionSchema);

  const { name } = pipe(
    useLiveQuery(
      (_) =>
        _.from({ item: httpCollection })
          .where((_) => eq(_.item.httpId, httpId))
          .select((_) => pick(_.item, 'name'))
          .findOne(),
      [httpCollection, httpId],
    ),
    (_) => Option.fromNullable(_.data),
    Option.getOrThrow,
  );

  const searchParamCollection = useApiCollection(HttpSearchParamCollectionSchema);

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => httpCollection.utils.update({ httpId, name: _ }),
    value: name,
  });

  const [isSending, startTransition] = useTransition();

  return (
    <>
      <div className='flex items-center gap-2 border-b border-slate-200 px-4 py-2.5'>
        <div
          className={tw`
            flex min-w-0 flex-1 gap-1 text-md leading-5 font-medium tracking-tight text-slate-400 select-none
          `}
        >
          {/* {example.breadcrumbs.map((_, index) => {
            // TODO: add links to breadcrumbs
            const key = enumToString(ExampleBreadcrumbKindSchema, 'EXAMPLE_BREADCRUMB_KIND', _.kind);
            const name = _[key]?.name;
            return (
              <Fragment key={`${index} ${name}`}>
                <span>{name}</span>
                <span>/</span>
              </Fragment>
            );
          })} */}

          {isEditing ? (
            <TextInputField
              aria-label='Example name'
              inputClassName={tw`-my-1 py-1 leading-none text-slate-800`}
              {...textFieldProps}
            />
          ) : (
            <AriaButton
              className={tw`max-w-full cursor-text truncate text-slate-800`}
              onContextMenu={onContextMenu}
              onPress={() => void edit()}
            >
              {name}
            </AriaButton>
          )}
        </div>

        <DialogTrigger>
          <Button className={tw`px-2 py-1 text-slate-800`} variant='ghost'>
            <FiClock className={tw`size-4 text-slate-500`} /> Response History
          </Button>

          <HistoryModal httpId={httpId} />
        </DialogTrigger>

        <MenuTrigger {...menuTriggerProps}>
          <Button className={tw`p-1`} variant='ghost'>
            <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
          </Button>

          <Menu {...menuProps}>
            <MenuItem onAction={() => void edit()}>Rename</MenuItem>

            <MenuItem onAction={() => void httpCollection.utils.delete({ httpId })} variant='danger'>
              Delete
            </MenuItem>
          </Menu>
        </MenuTrigger>
      </div>

      <div className={tw`flex gap-3 p-6 pb-0`}>
        <HttpUrl httpId={httpId} />

        <Button
          className={tw`px-6`}
          isPending={isSending}
          onPress={() =>
            void startTransition(async () => {
              const httpTransactions = Array.fromIterable(httpCollection._state.transactions.values());
              const searchParamTransactions = Array.fromIterable(searchParamCollection._state.transactions.values());

              await pipe(
                Array.appendAll(httpTransactions, searchParamTransactions),
                Array.map((_) => _.isPersisted.promise),
                (_) => Promise.all(_),
              );

              await Connect.request({ input: { httpId }, method: HttpService.method.httpRun, transport });
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
