import { Array, pipe } from 'effect';
import { useTransition } from 'react';
import { Button as AriaButton, DialogTrigger, MenuTrigger } from 'react-aria-components';
import { FiClock, FiMoreHorizontal } from 'react-icons/fi';
import { HttpService } from '@the-dev-tools/spec/buf/api/http/v1/http_pb';
import {
  HttpCollectionSchema,
  HttpDeltaCollectionSchema,
  HttpSearchParamCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { Button } from '@the-dev-tools/ui/button';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { Connect, useApiCollection } from '~/api';
import { rootRouteApi } from '~/routes';
import { DeltaResetButton, useDeltaState } from '~/utils/delta';
import { HistoryModal } from '../history';
import { HttpUrl } from './url';

export interface HttpTopBarProps {
  deltaHttpId: Uint8Array | undefined;
  httpId: Uint8Array;
}

export const HttpTopBar = ({ deltaHttpId, httpId }: HttpTopBarProps) => {
  const { transport } = rootRouteApi.useRouteContext();

  const collection = useApiCollection(HttpCollectionSchema);
  const deltaCollection = useApiCollection(HttpDeltaCollectionSchema);

  const deltaOptions = {
    deltaId: deltaHttpId,
    deltaSchema: HttpDeltaCollectionSchema,
    isDelta: deltaHttpId !== undefined,
    originId: httpId,
    originSchema: HttpCollectionSchema,
  };

  const [name, setName] = useDeltaState({ ...deltaOptions, valueKey: 'name' });

  const searchParamCollection = useApiCollection(HttpSearchParamCollectionSchema);

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

          <DeltaResetButton {...deltaOptions} valueKey='name' />
        </div>

        <DialogTrigger>
          <Button className={tw`px-2 py-1 text-slate-800`} variant='ghost'>
            <FiClock className={tw`size-4 text-slate-500`} /> Response History
          </Button>

          <HistoryModal deltaHttpId={deltaHttpId} httpId={httpId} />
        </DialogTrigger>

        <MenuTrigger {...menuTriggerProps}>
          <Button className={tw`p-1`} variant='ghost'>
            <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
          </Button>

          <Menu {...menuProps}>
            <MenuItem onAction={() => void edit()}>Rename</MenuItem>

            <MenuItem
              onAction={() => {
                if (deltaHttpId) deltaCollection.utils.delete({ deltaHttpId });
                else collection.utils.delete({ httpId });
              }}
              variant='danger'
            >
              Delete
            </MenuItem>
          </Menu>
        </MenuTrigger>
      </div>

      <div className={tw`flex gap-3 p-6 pb-0`}>
        <HttpUrl deltaHttpId={deltaHttpId} httpId={httpId} />

        <Button
          className={tw`px-6`}
          isPending={isSending}
          onPress={() =>
            void startTransition(async () => {
              const httpTransactions = Array.fromIterable(collection._state.transactions.values());
              const searchParamTransactions = Array.fromIterable(searchParamCollection._state.transactions.values());

              await pipe(
                Array.appendAll(httpTransactions, searchParamTransactions),
                Array.map((_) => _.isPersisted.promise),
                (_) => Promise.all(_),
              );

              await Connect.request({ input: { httpId: deltaHttpId ?? httpId }, method: HttpService.method.httpRun, transport });
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
