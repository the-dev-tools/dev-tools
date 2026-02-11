import { Tooltip, TooltipTrigger } from 'react-aria-components';
import { LuTrash2 } from 'react-icons/lu';
import { Button } from '@the-dev-tools/ui/button';
import { tw } from '@the-dev-tools/ui/tailwind-literal';

interface ColumnActionDeleteProps {
  onDelete: () => void;
}

export const ColumnActionDelete = ({ onDelete }: ColumnActionDeleteProps) => (
  <TooltipTrigger delay={750}>
    <Button className={tw`p-1 text-red-700`} onPress={onDelete} variant='ghost'>
      <LuTrash2 />
    </Button>
    <Tooltip className={tw`rounded-md bg-slate-800 px-2 py-1 text-xs text-white`}>Delete</Tooltip>
  </TooltipTrigger>
);
