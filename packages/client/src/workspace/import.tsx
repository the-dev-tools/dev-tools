import { MessageInitShape } from '@bufbuild/protobuf';
import { useNavigate } from '@tanstack/react-router';
import { Array, Option, pipe } from 'effect';
import { Ulid } from 'id128';
import { ReactNode, useState, useTransition } from 'react';
import {
  Cell,
  Column,
  Dialog,
  Heading,
  Row,
  Selection,
  Table,
  TableBody,
  TableHeader,
  Tooltip,
  TooltipTrigger,
} from 'react-aria-components';
import { FiInfo, FiX } from 'react-icons/fi';
import { twMerge } from 'tailwind-merge';
import { ImportKind, ImportRequestSchema } from '@the-dev-tools/spec/import/v1/import_pb';
import { ExampleListEndpoint } from '@the-dev-tools/spec/meta/collection/item/example/v1/example.endpoints.js';
import { CollectionItemListEndpoint } from '@the-dev-tools/spec/meta/collection/item/v1/item.endpoints.js';
import { CollectionListEndpoint } from '@the-dev-tools/spec/meta/collection/v1/collection.endpoints.js';
import { FlowListEndpoint } from '@the-dev-tools/spec/meta/flow/v1/flow.endpoints.js';
import { ImportEndpoint } from '@the-dev-tools/spec/meta/import/v1/import.endpoints.ts';
import { Button } from '@the-dev-tools/ui/button';
import { Checkbox } from '@the-dev-tools/ui/checkbox';
import { tableStyles } from '@the-dev-tools/ui/data-table';
import { FileDropZone } from '@the-dev-tools/ui/file-drop-zone';
import { FileImportIcon } from '@the-dev-tools/ui/icons';
import { Modal, useProgrammaticModal } from '@the-dev-tools/ui/modal';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField } from '@the-dev-tools/ui/text-field';
import { matchAllEndpoint, setQueryChild } from '~data-client';
import { flowLayoutRouteApi, rootRouteApi, workspaceRouteApi } from '~routes';

export const ImportDialog = () => {
  const { dataClient } = rootRouteApi.useRouteContext();

  const modal = useProgrammaticModal();

  const successAction = async () => {
    modal.onOpenChange(false);
    await dataClient.controller.expireAll({
      testKey: (_) => {
        if (matchAllEndpoint(CollectionListEndpoint)(_)) return true;
        if (matchAllEndpoint(CollectionItemListEndpoint)(_)) return true;
        if (matchAllEndpoint(ExampleListEndpoint)(_)) return true;
        return false;
      },
    });
  };

  return (
    <>
      <TooltipTrigger delay={750}>
        <Button
          className={tw`p-1`}
          onPress={() =>
            void modal.onOpenChange(
              true,
              <InitialDialog setModal={(node) => void modal.onOpenChange(true, node)} successAction={successAction} />,
            )
          }
          variant='ghost'
        >
          <FileImportIcon className={tw`size-4 text-slate-500`} />
        </Button>
        <Tooltip className={tw`rounded-md bg-slate-800 px-2 py-1 text-xs text-white`}>
          Import Collections and Flows
        </Tooltip>
      </TooltipTrigger>

      {modal.children && (
        <Modal
          {...modal}
          style={{ maxHeight: 'max(40vh, min(32rem, 90vh))', maxWidth: 'max(40vw, min(40rem, 90vw))' }}
        />
      )}
    </>
  );
};

interface InnerDialogProps {
  action: ReactNode;
  children: ReactNode;
}

const InnerDialog = ({ action, children }: InnerDialogProps) => (
  <Dialog className={tw`flex h-full flex-col overflow-auto outline-hidden`}>
    {({ close }) => (
      <>
        <div className={tw`flex h-full min-h-0 flex-1 flex-col overflow-auto p-6`}>
          <div className={tw`flex items-center justify-between`}>
            <Heading className={tw`text-xl leading-6 font-semibold tracking-tighter text-slate-800`} slot='title'>
              Import Collections and Flows
            </Heading>

            <Button className={tw`p-1`} onPress={() => void close()} variant='ghost'>
              <FiX className={tw`size-5 text-slate-500`} />
            </Button>
          </div>

          {children}
        </div>

        <div className={tw`flex justify-end gap-2 border-t border-slate-200 px-6 py-3`}>
          <Button onPress={() => void close()}>Cancel</Button>

          {action}
        </div>
      </>
    )}
  </Dialog>
);

interface InitialDialogProps {
  setModal: (node: ReactNode) => void;
  successAction: () => Promise<void>;
}

const InitialDialog = ({ setModal, successAction }: InitialDialogProps) => {
  const { dataClient } = rootRouteApi.useRouteContext();
  const { workspaceId } = workspaceRouteApi.useLoaderData();

  const [text, setText] = useState('');
  const [files, setFiles] = useState<File[]>();
  const file = files?.[0];
  const [isPending, startTransition] = useTransition();

  const data = pipe(
    Option.fromNullable(files?.[0]),
    Option.map((_) => _.arrayBuffer().then((_) => new Uint8Array(_))),
    Option.getOrElse(() => Promise.resolve(undefined)),
  );

  const importAction = async () => {
    const input: MessageInitShape<typeof ImportRequestSchema> = {
      data: (await data) ?? new Uint8Array(),
      name: file?.name ?? '',
      textData: text,
      workspaceId,
    };

    const result = await dataClient.fetch(ImportEndpoint, input);

    if (result.kind === ImportKind.FILTER)
      setModal(<DomainDialog domains={result.filter} input={input} successAction={successAction} />);
    else await successAction();
  };

  return (
    <InnerDialog
      action={
        <Button
          isDisabled={!files?.length && !text}
          isPending={isPending}
          onPress={() => void startTransition(importAction)}
          variant='primary'
        >
          Import
        </Button>
      }
    >
      <div
        className={tw`
          mt-6 rounded-lg border border-slate-200 bg-slate-50 p-4 text-sm leading-4 font-medium tracking-tight
          text-slate-500
        `}
      >
        <FiInfo className={tw`mr-1.5 inline-block size-4 align-bottom`} />
        Import Postman or HAR files
      </div>

      <TextInputField
        className={tw`mt-4`}
        label='Text value'
        onChange={setText}
        placeholder='Paste cURL, Raw text or URL...'
        value={text}
      />

      <FileDropZone className={tw`mt-4 flex-1`} files={files} onChange={setFiles} />
    </InnerDialog>
  );
};

interface DomainDialogProps {
  domains: string[];
  input: MessageInitShape<typeof ImportRequestSchema>;
  successAction: () => Promise<void>;
}

const DomainDialog = ({ domains, input, successAction }: DomainDialogProps) => {
  const { dataClient, transport } = rootRouteApi.useRouteContext();
  const { workspaceId } = workspaceRouteApi.useLoaderData();

  const navigate = useNavigate();

  const [selection, setSelection] = useState<Selection>('all');
  const [isPending, startTransition] = useTransition();

  const importAction = async () => {
    const finalFilters =
      selection === 'all'
        ? domains
        : pipe(
            selection.values(),
            Array.fromIterable,
            Array.filterMap((_) => Option.fromNullable(domains[_ as number])),
          );

    const { flow } = await dataClient.fetch(ImportEndpoint, {
      ...input,
      filter: finalFilters,
      kind: ImportKind.FILTER,
    });

    if (flow) {
      await setQueryChild(
        dataClient.controller,
        FlowListEndpoint.schema.items,
        'push',
        { controller: () => dataClient.controller, input: { workspaceId }, transport },
        flow,
      );

      const flowIdCan = Ulid.construct(flow.flowId).toCanonical();

      await navigate({
        from: workspaceRouteApi.id,
        to: flowLayoutRouteApi.id,

        params: { flowIdCan },
      });
    }

    await successAction();
  };

  return (
    <InnerDialog
      action={
        <Button
          isDisabled={selection !== 'all' && selection.size === 0}
          isPending={isPending}
          onPress={() => void startTransition(importAction)}
          variant='primary'
        >
          Import
        </Button>
      }
    >
      <div className={tw`text-xs leading-5 tracking-tight text-slate-500`}>
        Please deselect the domain names to be excluded in the flow. There might be requests that you may not want to
        import.
      </div>

      <div className={twMerge(tableStyles().container(), tw`mt-4 flex-1`)}>
        <Table
          aria-label='Filters'
          className={twMerge(tableStyles().base(), tw`grid-cols-[auto_1fr]`)}
          onSelectionChange={setSelection}
          selectedKeys={selection}
          selectionMode='multiple'
        >
          <TableHeader className={twMerge(tableStyles().header(), tw`sticky top-0 z-10`)}>
            <Column className={twMerge(tableStyles().headerColumn(), tw`!border-r-0 px-2`)}>
              <Checkbox isTableCell slot='selection' />
            </Column>
            <Column className={tableStyles().headerColumn()} isRowHeader>
              Domain
            </Column>
          </TableHeader>

          <TableBody className={tableStyles().body()} items={domains.map((value, index) => ({ index, value }))}>
            {(_) => (
              <Row className={twMerge(tableStyles().row(), tw`cursor-pointer`)} id={_.index}>
                <Cell className={twMerge(tableStyles().cell(), tw`!border-r-0`)}>
                  <div className={tw`flex justify-center`}>
                    <Checkbox isTableCell slot='selection' />
                  </div>
                </Cell>
                <Cell className={twMerge(tableStyles().cell(), tw`px-5 py-1.5`)}>{_.value}</Cell>
              </Row>
            )}
          </TableBody>
        </Table>
      </div>
    </InnerDialog>
  );
};
