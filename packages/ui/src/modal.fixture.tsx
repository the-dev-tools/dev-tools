import { Dialog, DialogTrigger } from 'react-aria-components';

import { Button } from './button';
import { Modal } from './modal';
import { tw } from './tailwind-literal';

export default (
  <DialogTrigger>
    <Button>Open Modal</Button>

    <Modal>
      <Dialog className={tw`outline-hidden`}>
        <h1 className={tw`mb-1 leading-5 font-semibold tracking-tight text-slate-800`}>Delete workspace?</h1>
        <span className={tw`text-sm leading-5 font-medium tracking-tight text-slate-500`}>
          This action will remove the workspace permanently
        </span>

        <div className={tw`mt-5 flex justify-end gap-2`}>
          <Button variant='secondary'>Cancel</Button>
          <Button className={tw`border-red-700 bg-red-600`} variant='primary'>
            Delete
          </Button>
        </div>
      </Dialog>
    </Modal>
  </DialogTrigger>
);
