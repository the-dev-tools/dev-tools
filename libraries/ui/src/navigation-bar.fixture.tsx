import { FiBell, FiHelpCircle, FiSearch, FiSettings } from 'react-icons/fi';

import { RootDecoratorOptions } from '../cosmos.decorator';
import { AddButton } from './add-button';
import { Avatar } from './avatar';
import { Logo } from './illustrations';
import { NavigationBar, NavigationBarDivider } from './navigation-bar';
import { tw } from './tailwind-literal';

export const options: RootDecoratorOptions = { isCentered: false };

export default (
  <NavigationBar>
    <Logo className='size-7' />
    <NavigationBarDivider />
    <span>Home</span>
    <div className='flex-1' />
    <div className='flex gap-1.5'>
      <Avatar variant='lime'>A</Avatar>
      <Avatar variant='amber'>B</Avatar>
      <Avatar variant='blue'>C</Avatar>
      <Avatar shorten={false} variant='violet'>
        3+
      </Avatar>
      <AddButton variant='light' />
    </div>
    <NavigationBarDivider />
    <div className='flex gap-1'>
      <FiSearch className={tw`size-5 stroke-[1.2px] text-slate-400`} />
      <FiHelpCircle className={tw`size-5 stroke-[1.2px] text-slate-400`} />
      <FiSettings className={tw`size-5 stroke-[1.2px] text-slate-400`} />
      <FiBell className={tw`size-5 stroke-[1.2px] text-slate-400`} />
    </div>
    <NavigationBarDivider />
    <Avatar variant='teal'>User</Avatar>
  </NavigationBar>
);
