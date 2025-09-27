import { Array, Option, pipe, Predicate, Schema } from 'effect';
import { Ulid } from 'id128';
import { useState } from 'react';
import { Dialog, DialogTrigger, Key, ListBox, Selection, Tooltip, TooltipTrigger } from 'react-aria-components';
import { FiX } from 'react-icons/fi';
import { TbFileExport } from 'react-icons/tb';
import { export$ } from '@the-dev-tools/spec/export/v1/export-ExportService_connectquery';
import { FlowListEndpoint } from '@the-dev-tools/spec/meta/flow/v1/flow.endpoints.js';
import { Button } from '@the-dev-tools/ui/button';
import { CollectionIcon, FlowsIcon } from '@the-dev-tools/ui/icons';
import { ListBoxItem } from '@the-dev-tools/ui/list-box';
import { Modal } from '@the-dev-tools/ui/modal';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { saveFile } from '@the-dev-tools/ui/utils';
import { useConnectMutation } from '~api/connect-query';
import { CollectionListTree, TreeKey } from '~collection';
import { useQuery } from '~data-client';
import { workspaceRouteApi } from '~routes';

export const ExportDialog = () => {
  const { workspaceId } = workspaceRouteApi.useLoaderData();

  const { items: flows } = useQuery(FlowListEndpoint, { workspaceId });

  const [isOpen, setOpen] = useState(false);
  const [collectionSelection, setCollectionSelection] = useState<Selection>(new Set());
  const [flowSelection, setFlowSelection] = useState<Selection>(new Set());

  const onOpenChange = (isOpen: boolean) => {
    setOpen(isOpen);
    if (!isOpen) return;
    setCollectionSelection(new Set());
    setFlowSelection(new Set());
  };

  // TODO: switch to Data Client Endpoint
  const exportMutation = useConnectMutation(export$);

  return (
    <DialogTrigger isOpen={isOpen} onOpenChange={onOpenChange}>
      <TooltipTrigger delay={750}>
        <Button className={tw`p-1`} variant='ghost'>
          <TbFileExport className={tw`size-4 text-slate-500`} />
        </Button>
        <Tooltip className={tw`rounded-md bg-slate-800 px-2 py-1 text-xs text-white`}>
          Export Requests and Flows
        </Tooltip>
      </TooltipTrigger>

      <Modal style={{ maxHeight: 'max(40vh, min(32rem, 90vh))', maxWidth: 'max(40vw, min(40rem, 90vw))' }}>
        <Dialog className={tw`flex h-full flex-col overflow-auto outline-hidden`}>
          <div className={tw`flex h-full min-h-0 flex-1 flex-col overflow-auto p-6`}>
            <div className={tw`flex items-center justify-between`}>
              <div className={tw`text-xl leading-6 font-semibold tracking-tighter text-slate-800`}>
                Export Requests and Flows
              </div>

              <Button className={tw`p-1`} onPress={() => void onOpenChange(false)} variant='ghost'>
                <FiX className={tw`size-5 text-slate-500`} />
              </Button>
            </div>

            <div className={tw`text-xs leading-5 tracking-tight text-slate-500`}>
              Please select the requests and/or flows that you would like to export.
            </div>

            <div className={tw`flex flex-1 flex-col gap-2 overflow-auto py-1.5`}>
              <div className={tw`flex items-center gap-2 px-2.5 py-1.5`}>
                <CollectionIcon className={tw`size-5 text-slate-500`} />
                <h2 className={tw`flex-1 text-md leading-5 font-semibold tracking-tight text-slate-800`}>
                  Collections
                </h2>
              </div>

              <CollectionListTree
                onSelectionChange={(selection) => {
                  console.log(selection);
                  if (selection === 'all') return;
                  const selectedExampleKeys = pipe(
                    Array.fromIterable(selection),
                    Array.filter((keyUnknown) => {
                      if (typeof keyUnknown !== 'string') return false;
                      const key = pipe(Schema.parseJson(TreeKey), Schema.decodeUnknownSync, (_) => _(keyUnknown));
                      const exampleKeyTags: (typeof TreeKey.Type._tag)[] = ['EndpointKey', 'ExampleKey'];
                      return exampleKeyTags.includes(key._tag);
                    }),
                  );
                  setCollectionSelection(new Set(selectedExampleKeys));
                }}
                selectedKeys={collectionSelection}
                selectionMode='multiple'
              />

              <div className={tw`flex items-center gap-2 px-2.5 py-1.5`}>
                <FlowsIcon className={tw`size-5 text-slate-500`} />
                <h2 className={tw`flex-1 text-md leading-5 font-semibold tracking-tight text-slate-800`}>Flows</h2>
              </div>

              <ListBox
                aria-label='Flow list'
                className={tw`w-full`}
                items={flows}
                onSelectionChange={setFlowSelection}
                selectedKeys={flowSelection}
                selectionMode='multiple'
              >
                {(_) => (
                  <ListBoxItem
                    className={tw`rounded-md pl-9 text-md leading-5 font-medium`}
                    id={Ulid.construct(_.flowId).toCanonical()}
                  >
                    {_.name}
                  </ListBoxItem>
                )}
              </ListBox>
            </div>
          </div>

          <div className={tw`flex justify-end gap-2 border-t border-slate-200 px-6 py-3`}>
            <Button onPress={() => void onOpenChange(false)}>Cancel</Button>

            <Button
              isDisabled={
                collectionSelection !== 'all' &&
                collectionSelection.size === 0 &&
                flowSelection !== 'all' &&
                flowSelection.size === 0
              }
              onPress={async () => {
                const exampleIds = pipe(
                  Option.liftPredicate(collectionSelection, (_) => _ !== 'all'),
                  Option.map(Array.fromIterable),
                  Option.getOrElse(Array.empty<Key>),
                  Array.filterMap((keyUnknown) => {
                    const key = pipe(Schema.parseJson(TreeKey), Schema.decodeUnknownSync, (_) => _(keyUnknown));
                    if ('exampleId' in key) return Option.some(key.exampleId);
                    return Option.none();
                  }),
                );

                const flowIds = pipe(
                  Option.liftPredicate(flowSelection, (_) => _ !== 'all'),
                  Option.map(Array.fromIterable),
                  Option.getOrElse(Array.empty<Key>),
                  Array.filterMap((_) =>
                    pipe(
                      Option.liftPredicate(_, Predicate.isString),
                      Option.map((_) => Ulid.fromCanonical(_).bytes),
                    ),
                  ),
                );

                const { data, name } = await exportMutation.mutateAsync({ exampleIds, flowIds, workspaceId });
                saveFile({ blobParts: [data], name });
                onOpenChange(false);
              }}
              variant='primary'
            >
              Export
            </Button>
          </div>
        </Dialog>
      </Modal>
    </DialogTrigger>
  );
};
