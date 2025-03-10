import { getRouteApi } from '@tanstack/react-router';
import { Array, Option, pipe } from 'effect';
import { useState } from 'react';
import {
  Cell,
  Column,
  Dialog,
  DialogTrigger,
  Row,
  Selection,
  Table,
  TableBody,
  TableHeader,
} from 'react-aria-components';
import { FiInfo, FiX } from 'react-icons/fi';
import { twMerge } from 'tailwind-merge';

import { useConnectMutation } from '@the-dev-tools/api/connect-query';
import { ImportKind } from '@the-dev-tools/spec/import/v1/import_pb';
import { import$ } from '@the-dev-tools/spec/import/v1/import-ImportService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { Checkbox } from '@the-dev-tools/ui/checkbox';
import { tableStyles } from '@the-dev-tools/ui/data-table';
import { FileDropZone } from '@the-dev-tools/ui/file-drop-zone';
import { FileImportIcon } from '@the-dev-tools/ui/icons';
import { Modal } from '@the-dev-tools/ui/modal';
import { tw } from '@the-dev-tools/ui/tailwind-literal';

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceIdCan');

export const ImportDialog = () => {
  const { workspaceId } = workspaceRoute.useLoaderData();

  const [isOpen, setOpen] = useState(false);
  const [files, setFiles] = useState<File[]>();
  const [filters, setFilters] = useState<string[]>();
  const [selectedFilters, setSelectedFilters] = useState<Selection>(new Set());

  const isFilterSelected = selectedFilters === 'all' || selectedFilters.size > 0;

  const importMutation = useConnectMutation(import$);

  const onOpenChange = (isOpen: boolean) => {
    setOpen(isOpen);
    if (!isOpen) return;
    setFiles(undefined);
    setFilters(undefined);
    setSelectedFilters(new Set());
  };

  const importUniversal = !filters && (
    <>
      <div
        className={tw`mt-6 rounded-lg border border-slate-200 bg-slate-50 p-4 text-sm font-medium leading-4 tracking-tight text-slate-500`}
      >
        <FiInfo className={tw`mr-1.5 inline-block size-4 align-bottom`} />
        Import Postman or HAR files
      </div>

      {/* <TextField className={tw`mt-4`} inputPlaceholder='Paste cURL, Raw text or URL...' /> */}

      <FileDropZone dropZoneClassName={tw`mt-4 flex-1`} onChange={setFiles} files={files} />
    </>
  );

  const importUniversalSubmit = !filters && (
    <Button
      variant='primary'
      isDisabled={!files?.length}
      onPress={async () => {
        const file = files?.[0];
        if (!file) return;

        const data = pipe(await file.arrayBuffer(), (_) => new Uint8Array(_));

        const result = await importMutation.mutateAsync({
          workspaceId,
          name: file.name,
          data,
        });

        if (result.kind === ImportKind.FILTER) setFilters(result.filter);
        else onOpenChange(false);
      }}
    >
      Import
    </Button>
  );

  const importFilter = filters && (
    <>
      <div className={tw`text-xs leading-5 tracking-tight text-slate-500`}>
        Please deselect the domain names to be excluded in the flow. There might be requests that you may not want to
        import.
      </div>

      <div className={twMerge(tableStyles.wrapper, tw`mt-4 flex-1`)}>
        <Table
          selectedKeys={selectedFilters}
          onSelectionChange={setSelectedFilters}
          aria-label='Filters'
          selectionMode='multiple'
          className={tableStyles.table}
        >
          <TableHeader className={twMerge(tableStyles.header, tableStyles.row, tw`sticky top-0 z-10`)}>
            <Column className={twMerge(tableStyles.headerCell, tw`!w-0 min-w-0 px-2`)}>
              <Checkbox variant='table-cell' slot='selection' />
            </Column>
            <Column className={tableStyles.headerCell} isRowHeader>
              Domain
            </Column>
          </TableHeader>

          <TableBody items={filters.map((value, index) => ({ value, index }))} className={tableStyles.body}>
            {(_) => (
              <Row id={_.index} className={twMerge(tableStyles.row, tw`cursor-pointer`)}>
                <Cell className={tableStyles.cell}>
                  <div className={tw`flex justify-center`}>
                    <Checkbox variant='table-cell' slot='selection' />
                  </div>
                </Cell>
                <Cell className={twMerge(tableStyles.cell, tw`!border-l-0 px-5 py-1.5`)}>{_.value}</Cell>
              </Row>
            )}
          </TableBody>
        </Table>
      </div>
    </>
  );

  const importFilterSubmit = filters && (
    <Button
      variant='primary'
      isDisabled={!isFilterSelected}
      onPress={async () => {
        const file = files?.[0];
        if (!file) return;

        const data = pipe(await file.arrayBuffer(), (_) => new Uint8Array(_));

        const finalFilters =
          selectedFilters === 'all'
            ? filters
            : pipe(
                selectedFilters.values(),
                Array.fromIterable,
                Array.filterMap((_) => Option.fromNullable(filters[_ as number])),
              );

        await importMutation.mutateAsync({
          kind: ImportKind.FILTER,
          workspaceId,
          name: file.name,
          data,
          filter: finalFilters,
        });

        onOpenChange(false);
      }}
    >
      Import
    </Button>
  );

  return (
    <DialogTrigger isOpen={isOpen} onOpenChange={onOpenChange}>
      <Button className={tw`p-1`} variant='ghost'>
        <FileImportIcon className={tw`size-4 text-slate-500`} />
      </Button>

      <Modal modalSize='sm'>
        <Dialog className={tw`flex h-full flex-col overflow-auto outline-none`}>
          <div className={tw`flex h-full min-h-0 flex-1 flex-col overflow-auto p-6`}>
            <div className={tw`flex items-center justify-between`}>
              <div className={tw`text-xl font-semibold leading-6 tracking-tighter text-slate-800`}>
                Import Collections and Flows
              </div>

              <Button className={tw`p-1`} variant='ghost' onPress={() => void onOpenChange(false)}>
                <FiX className={tw`size-5 text-slate-500`} />
              </Button>
            </div>

            {importUniversal}
            {importFilter}
          </div>

          <div className={tw`flex justify-end gap-2 border-t border-slate-200 px-6 py-3`}>
            <Button onPress={() => void onOpenChange(false)}>Cancel</Button>

            {importUniversalSubmit}
            {importFilterSubmit}
          </div>
        </Dialog>
      </Modal>
    </DialogTrigger>
  );
};
