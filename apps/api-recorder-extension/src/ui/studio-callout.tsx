import * as RAC from 'react-aria-components';
import * as FeatherIcons from 'react-icons/fi';
import { twMerge } from 'tailwind-merge';

import { buttonStyles } from '~/ui/button';
import { STUDIO_CTA, STUDIO_DOWNLOAD_URL, STUDIO_NAME } from '~brand';

export interface StudioCalloutProps {
  className?: string;
  /** File name of the last export, shown as a confirmation before the suggestion */
  exportedFile?: string | undefined;
}

/** One friendly line suggesting to open recordings in Studio, with a download link */
export const StudioCallout = ({ className, exportedFile }: StudioCalloutProps) => (
  <div
    aria-live='polite'
    className={twMerge(
      'flex items-center gap-3 border-indigo-100 bg-indigo-50 px-4 py-2.5 text-sm leading-5 text-slate-700',
      className,
    )}
  >
    {exportedFile ? (
      <FeatherIcons.FiCheckCircle aria-hidden className='size-5 shrink-0 text-green-600' />
    ) : (
      <FeatherIcons.FiDownload aria-hidden className='size-5 shrink-0 text-indigo-600' />
    )}
    <p className='min-w-0 flex-1'>
      {exportedFile && (
        <span className='font-medium text-slate-900'>
          Saved <span className='font-mono text-xs'>{exportedFile}</span>.{' '}
        </span>
      )}
      {STUDIO_CTA}
    </p>
    <RAC.Link
      aria-label={`Download ${STUDIO_NAME}`}
      className={buttonStyles({ className: 'shrink-0 px-3 py-2 text-sm', variant: 'primary' })}
      href={STUDIO_DOWNLOAD_URL}
      rel='noreferrer'
      target='_blank'
    >
      Download
    </RAC.Link>
  </div>
);
