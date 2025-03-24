import { Dialog, DialogTrigger } from 'react-aria-components';

import { Button } from './button';
import { Modal } from './modal';
import { tw } from './tailwind-literal';

export default (
  <DialogTrigger>
    <Button>Open Modal</Button>

    <Modal>
      <Dialog className={tw`outline-hidden`}>
        <h1 className={tw`mb-1 font-semibold leading-5 tracking-tight text-slate-800`}>Delete workspace?</h1>
        <span className={tw`text-sm font-medium leading-5 tracking-tight text-slate-500`}>
          This action will remove the workspace permanently
        </span>

        <div className={tw`mt-5 flex justify-end gap-2`}>
          <Button variant='secondary'>Cancel</Button>
          <Button variant='primary' className={tw`border-red-700 bg-red-600`}>
            Delete
          </Button>
        </div>
      </Dialog>
    </Modal>
  </DialogTrigger>
);
