import { SVGProps } from 'react';

export const CollectionIcon = (props: SVGProps<SVGSVGElement>) => (
  <svg xmlns='http://www.w3.org/2000/svg' width={18} height={18} fill='none' {...props}>
    <rect
      width={14}
      height={8}
      x={2}
      y={7}
      stroke='currentColor'
      strokeLinecap='round'
      strokeLinejoin='round'
      strokeWidth={1.5}
      rx={2}
    />
    <path
      stroke='currentColor'
      strokeLinecap='round'
      strokeLinejoin='round'
      strokeWidth={1.5}
      d='M7 10h3.5M3.5 7V6a2 2 0 0 1 2-2h7a2 2 0 0 1 2 2v1'
    />
  </svg>
);

export const FlowsIcon = (props: SVGProps<SVGSVGElement>) => (
  <svg xmlns='http://www.w3.org/2000/svg' width={18} height={18} fill='none' {...props}>
    <path stroke='currentColor' strokeWidth={1.5} d='M12.5 5H10a1 1 0 0 0-1 1v6.1a1 1 0 0 0 1 1h2.5M6 9h3' />
    <rect width={4} height={4} x={2} y={7} stroke='currentColor' strokeWidth={1.5} rx={1} />
    <rect width={4} height={4} x={12} y={3.15} stroke='currentColor' strokeWidth={1.5} rx={1} />
    <rect width={4} height={4} x={12} y={11.25} stroke='currentColor' strokeWidth={1.5} rx={1} />
  </svg>
);

export const OverviewIcon = (props: SVGProps<SVGSVGElement>) => (
  <svg xmlns='http://www.w3.org/2000/svg' width={18} height={18} fill='none' {...props}>
    <g stroke='currentColor' strokeLinecap='round' strokeLinejoin='round' strokeWidth={1.5} clipPath='url(#a)'>
      <path d='M6 3.75H4.5A1.5 1.5 0 0 0 3 5.25v9a1.5 1.5 0 0 0 1.5 1.5h4.273M13.5 9V5.25a1.5 1.5 0 0 0-1.5-1.5h-1.5' />
      <path d='M6 3.75a1.5 1.5 0 0 1 1.5-1.5H9a1.5 1.5 0 0 1 0 3H7.5A1.5 1.5 0 0 1 6 3.75ZM6 8.25h3M6 11.25h2.25M10.5 13.125a1.875 1.875 0 1 0 3.75 0 1.875 1.875 0 0 0-3.75 0ZM13.875 14.625 15.75 16.5' />
    </g>
    <defs>
      <clipPath id='a'>
        <path fill='#fff' d='M0 0h18v18H0z' />
      </clipPath>
    </defs>
  </svg>
);

export const FileImportIcon = (props: SVGProps<SVGSVGElement>) => (
  <svg xmlns='http://www.w3.org/2000/svg' width={16} height={16} fill='none' {...props}>
    <path
      stroke='currentColor'
      strokeLinecap='round'
      strokeWidth={1.2}
      d='M3.333 8V5l3-3h5.334a1 1 0 0 1 1 1v10a1 1 0 0 1-1 1H7.333M4.667 12H2'
    />
    <path
      stroke='currentColor'
      strokeLinecap='round'
      strokeLinejoin='round'
      strokeWidth={1.2}
      d='m5.333 12-1.666 1.333v-2.666L5.333 12Z'
    />
    <path stroke='currentColor' strokeLinecap='round' strokeWidth={1.2} d='M7.333 2.333V5a1 1 0 0 1-1 1H3.667' />
  </svg>
);

export const ChevronSolidDownIcon = (props: SVGProps<SVGSVGElement>) => (
  <svg xmlns='http://www.w3.org/2000/svg' width={12} height={12} fill='none' {...props}>
    <path
      fill='currentColor'
      d='m7.788 5.705-3.16-3.16a.417.417 0 0 0-.712.294V9.16c0 .372.449.558.711.295l3.161-3.16a.417.417 0 0 0 0-.59Z'
    />
  </svg>
);

export const FolderOpenedIcon = (props: SVGProps<SVGSVGElement>) => (
  <svg xmlns='http://www.w3.org/2000/svg' width='1em' height='1em' fill='none' viewBox='0 0 18 18' {...props}>
    <g stroke='currentColor' strokeLinecap='round' strokeLinejoin='round' strokeWidth={1.5} clipPath='url(#a)'>
      <path d='M15 7.5c0-1-.152-1.595-.423-1.87a1.433 1.433 0 0 0-1.021-.43H8.5L6.333 3H3.444c-.383 0-.75.155-1.02.43C2.151 3.705 2 4.078 2 4.467v9.066c0 .39.152.762.423 1.037.271.275.638.43 1.021.43h10.112' />
      <path d='M4.877 8.859A1 1 0 0 1 5.867 8h9.98a1 1 0 0 1 .99 1.141l-.714 5a1 1 0 0 1-.99.859H4l.877-6.141Z' />
    </g>
    <defs>
      <clipPath id='a'>
        <path fill='#fff' d='M0 0h18v18H0z' />
      </clipPath>
    </defs>
  </svg>
);
