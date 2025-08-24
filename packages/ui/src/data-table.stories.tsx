import type { Meta, StoryObj } from '@storybook/react-vite';
import { createColumnHelper, Table } from '@tanstack/react-table';
import { Array, pipe, Record } from 'effect';

import { DataTable, useReactTable } from './data-table';

type Item = Record<string, string>;

const columnHelper = createColumnHelper<Item>();

const columns = Array.makeBy(5, (column) => columnHelper.accessor(`col-${column}`, { header: `Column ${column + 1}` }));

const data: Item[] = Array.makeBy(10, (row) =>
  pipe(
    columns.map(({ accessorKey }, column) => [accessorKey, `${row + 1}:${column + 1}`] as const),
    Record.fromEntries,
  ),
);

const meta = {
  component: DataTable,

  args: { table: null as never },

  render: function Render(props) {
    const table = useReactTable({ columns, data });
    return <DataTable {...props} table={table as Table<object>} />;
  },
} satisfies Meta<typeof DataTable>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};
