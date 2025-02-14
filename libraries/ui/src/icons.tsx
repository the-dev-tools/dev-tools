import { SVGProps } from 'react';
import { twMerge } from 'tailwind-merge';

import { tw } from './tailwind-literal';

// Generated using this SVGR playground: https://react-svgr.com/playground/?exportType=named&icon=true&jsxRuntime=automatic&replaceAttrValues=%2364748B%3DcurrentColor&svgoConfig=%7B%0A%20%20%22plugins%22%3A%20%5B%0A%20%20%20%20%7B%0A%20%20%20%20%20%20%22name%22%3A%20%22preset-default%22%2C%0A%20%20%20%20%20%20%22params%22%3A%20%7B%0A%20%20%20%20%20%20%20%20%22overrides%22%3A%20%7B%0A%20%20%20%20%20%20%20%20%20%20%22removeTitle%22%3A%20false%2C%0A%20%20%20%20%20%20%20%20%20%20%22removeViewBox%22%3A%20false%0A%20%20%20%20%20%20%20%20%7D%0A%20%20%20%20%20%20%7D%0A%20%20%20%20%7D%0A%20%20%5D%0A%7D&typescript=true

export const CollectionIcon = (props: SVGProps<SVGSVGElement>) => (
  <svg xmlns='http://www.w3.org/2000/svg' width='1em' height='1em' fill='none' viewBox='0 0 18 18' {...props}>
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
  <svg xmlns='http://www.w3.org/2000/svg' width='1em' height='1em' fill='none' viewBox='0 0 18 18' {...props}>
    <path stroke='currentColor' strokeWidth={1.5} d='M12.5 5H10a1 1 0 0 0-1 1v6.1a1 1 0 0 0 1 1h2.5M6 9h3' />
    <rect width={4} height={4} x={2} y={7} stroke='currentColor' strokeWidth={1.5} rx={1} />
    <rect width={4} height={4} x={12} y={3.15} stroke='currentColor' strokeWidth={1.5} rx={1} />
    <rect width={4} height={4} x={12} y={11.25} stroke='currentColor' strokeWidth={1.5} rx={1} />
  </svg>
);

export const OverviewIcon = (props: SVGProps<SVGSVGElement>) => (
  <svg xmlns='http://www.w3.org/2000/svg' width='1em' height='1em' fill='none' viewBox='0 0 18 18' {...props}>
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
  <svg xmlns='http://www.w3.org/2000/svg' width='1em' height='1em' fill='none' viewBox='0 0 16 16' {...props}>
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
  <svg xmlns='http://www.w3.org/2000/svg' width='1em' height='1em' fill='none' viewBox='0 0 12 12' {...props}>
    <path
      fill='currentColor'
      d='m7.788 5.706-3.16-3.161a.417.417 0 0 0-.712.294v6.322c0 .371.449.557.711.295l3.161-3.161a.417.417 0 0 0 0-.59Z'
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

export const VariableIcon = (props: SVGProps<SVGSVGElement>) => (
  <svg xmlns='http://www.w3.org/2000/svg' width='1em' height='1em' fill='none' viewBox='0 0 16 16' {...props}>
    <path
      stroke='currentColor'
      strokeLinecap='round'
      strokeLinejoin='round'
      strokeWidth={1.2}
      d='M6 2H5a2 2 0 0 0-2 2v1.622a2 2 0 0 1-.918 1.683L1 8l1.082.695A2 2 0 0 1 3 10.378V12a2 2 0 0 0 2 2h1M10 2h1a2 2 0 0 1 2 2v1.622a2 2 0 0 0 .918 1.683L15 8l-1.082.695A2 2 0 0 0 13 10.378V12a2 2 0 0 1-2 2h-1M10 5l-4 6M10 11 6 5'
    />
  </svg>
);

export const GlobalEnvironmentIcon = (props: SVGProps<SVGSVGElement>) => (
  <svg xmlns='http://www.w3.org/2000/svg' width='1em' height='1em' fill='none' viewBox='0 0 16 16' {...props}>
    <path
      stroke='currentColor'
      strokeLinecap='round'
      strokeLinejoin='round'
      strokeWidth={1.2}
      d='M15.333 12.667c-.8 1.333-1.8 2-3 2s-2.2-.667-3-2c.8-1.334 1.8-2 3-2s2.2.666 3 2Z'
    />
    <circle cx={12.333} cy={12.667} r={0.667} fill='currentColor' />
    <path
      stroke='currentColor'
      strokeLinecap='round'
      strokeLinejoin='round'
      strokeWidth={1.2}
      d='M8 14H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h6.667a2 2 0 0 1 2 2v4.667M4 4.333h6.667M4 6.667h6.667M4 9h6.667M4 11.333h3.333'
    />
  </svg>
);

export const PlayIcon = (props: SVGProps<SVGSVGElement>) => (
  <svg xmlns='http://www.w3.org/2000/svg' width='1em' height='1em' fill='none' viewBox='0 0 16 16' {...props}>
    <path
      fill='currentColor'
      d='M12.57 6.717a1.455 1.455 0 0 1 0 2.566l-6.922 3.85c-1.026.57-2.308-.143-2.308-1.284V4.15c0-1.14 1.282-1.853 2.308-1.283l6.923 3.85Z'
    />
  </svg>
);

export const SendRequestIcon = (props: SVGProps<SVGSVGElement>) => (
  <svg xmlns='http://www.w3.org/2000/svg' width='1em' height='1em' fill='none' viewBox='0 0 16 16' {...props}>
    <path
      stroke='currentColor'
      strokeLinecap='round'
      strokeLinejoin='round'
      strokeWidth={1.2}
      d='M1.6 10.4v-4M4 6.4v4M1.6 8H4M5.6 6.4h1.6M8.8 6.4h1.6M6.4 6.4v4M9.6 6.4v4M12 8.4h1.2c.318 0 .623-.105.849-.293A.924.924 0 0 0 14.4 7.4a.924.924 0 0 0-.351-.707A1.331 1.331 0 0 0 13.2 6.4H12v4M3.2 4h9.6l-2.5-1.6M12.8 12.8H3.2l2.5 1.6'
    />
  </svg>
);

export const DataSourceIcon = (props: SVGProps<SVGSVGElement>) => (
  <svg xmlns='http://www.w3.org/2000/svg' width='1em' height='1em' fill='none' viewBox='0 0 16 16' {...props}>
    <path
      stroke='currentColor'
      strokeLinecap='round'
      strokeLinejoin='round'
      strokeWidth={1.2}
      d='M9.333 2v2.667a.667.667 0 0 0 .667.666h2.667'
    />
    <path
      stroke='currentColor'
      strokeLinecap='round'
      strokeLinejoin='round'
      strokeWidth={1.2}
      d='M11.333 14H4.667a1.334 1.334 0 0 1-1.334-1.333V3.333A1.333 1.333 0 0 1 4.667 2h4.666l3.334 3.333v7.334A1.334 1.334 0 0 1 11.333 14ZM6.667 8l2.666 3.333M6.667 11.333 9.333 8'
    />
  </svg>
);

export const DelayIcon = (props: SVGProps<SVGSVGElement>) => (
  <svg xmlns='http://www.w3.org/2000/svg' width='1em' height='1em' fill='none' viewBox='0 0 16 16' {...props}>
    <g stroke='currentColor' strokeLinecap='round' strokeLinejoin='round' strokeWidth={1.2} clipPath='url(#a)'>
      <path d='M4.333 4.667h7.334M4 13.333V12a4 4 0 1 1 8 0v1.333a.666.666 0 0 1-.667.667H4.667A.666.666 0 0 1 4 13.333Z' />
      <path d='M4 2.667V4a4 4 0 0 0 8 0V2.667A.666.666 0 0 0 11.333 2H4.667A.667.667 0 0 0 4 2.667Z' />
    </g>
    <defs>
      <clipPath id='a'>
        <path fill='#fff' d='M0 0h16v16H0z' />
      </clipPath>
    </defs>
  </svg>
);

export const IfIcon = (props: SVGProps<SVGSVGElement>) => (
  <svg xmlns='http://www.w3.org/2000/svg' width='1em' height='1em' fill='none' viewBox='0 0 16 16' {...props}>
    <g stroke='currentColor' strokeLinecap='round' strokeLinejoin='round' strokeWidth={1.2} clipPath='url(#a)'>
      <path d='M14 11.333H8.667L6.333 8H2M14 4.667H8.667L6.337 8' />
      <path d='m12 6.667 2-2-2-2M12 13.333l2-2-2-2' />
    </g>
    <defs>
      <clipPath id='a'>
        <path fill='#fff' d='M0 0h16v16H0z' />
      </clipPath>
    </defs>
  </svg>
);

export const ForIcon = (props: SVGProps<SVGSVGElement>) => (
  <svg xmlns='http://www.w3.org/2000/svg' width='1em' height='1em' fill='none' viewBox='0 0 16 16' {...props}>
    <path
      stroke='currentColor'
      strokeLinecap='round'
      strokeLinejoin='round'
      strokeWidth={1.2}
      d='M6.4 4H3.6a2 2 0 0 0-2 2v5.6a2 2 0 0 0 2 2h8.8a2 2 0 0 0 2-2V6a2 2 0 0 0-2-2H8.8'
    />
    <path
      stroke='currentColor'
      strokeLinecap='round'
      strokeLinejoin='round'
      strokeWidth={1.2}
      d='M4.8 2.4 6.4 4 4.8 5.6M8 9.6v1.6'
    />
    <circle cx={8} cy={7} r={0.5} fill='currentColor' stroke='currentColor' strokeWidth={0.2} />
  </svg>
);

export const CollectIcon = (props: SVGProps<SVGSVGElement>) => (
  <svg xmlns='http://www.w3.org/2000/svg' width='1em' height='1em' fill='none' viewBox='0 0 16 16' {...props}>
    <path
      stroke='currentColor'
      strokeLinecap='round'
      strokeLinejoin='round'
      strokeWidth={1.2}
      d='M6.4 3.2H3.6a2 2 0 0 0-2 2v5.6a2 2 0 0 0 2 2h8.8a2 2 0 0 0 2-2V5.2a2 2 0 0 0-2-2H8.8'
    />
    <path
      stroke='currentColor'
      strokeLinecap='round'
      strokeLinejoin='round'
      strokeWidth={1.2}
      d='m4.8 1.6 1.6 1.6-1.6 1.6'
    />
  </svg>
);

export const TextBoxIcon = (props: SVGProps<SVGSVGElement>) => (
  <svg xmlns='http://www.w3.org/2000/svg' width='1em' height='1em' fill='none' viewBox='0 0 20 20' {...props}>
    <path
      stroke='currentColor'
      strokeLinecap='round'
      strokeLinejoin='round'
      strokeWidth={1.2}
      d='M10.001 14.167V5.833m-3.333.834v-.833h6.667v.833m-4.167 7.5h1.667'
    />
    <rect
      width={14.167}
      height={14.167}
      x={2.918}
      y={2.917}
      stroke='currentColor'
      strokeLinecap='round'
      strokeLinejoin='round'
      strokeMiterlimit={1.5}
      strokeWidth={1.2}
      rx={2}
    />
  </svg>
);

export const ChatAddIcon = (props: SVGProps<SVGSVGElement>) => (
  <svg xmlns='http://www.w3.org/2000/svg' width='1em' height='1em' fill='none' viewBox='0 0 20 20' {...props}>
    <path
      stroke='currentColor'
      strokeWidth={1.2}
      d='M16.154 14.288a7.5 7.5 0 1 0-2.848 2.446c1.205.428 2.537.704 3.832.648.316-.014.473-.377.304-.645-.438-.693-.892-1.573-1.288-2.45Z'
    />
    <path
      stroke='currentColor'
      strokeLinecap='round'
      strokeLinejoin='round'
      strokeWidth={1.2}
      d='M10 7.5v5M12.5 10h-5'
    />
  </svg>
);

export const PlayCircleIcon = (props: SVGProps<SVGSVGElement>) => (
  <svg xmlns='http://www.w3.org/2000/svg' width='1em' height='1em' fill='none' viewBox='0 0 16 16' {...props}>
    <circle cx={8} cy={8} r={6} stroke='currentColor' strokeLinecap='round' strokeLinejoin='round' strokeWidth={1.2} />
    <path
      stroke='currentColor'
      strokeLinecap='round'
      strokeLinejoin='round'
      strokeWidth={1.2}
      d='m11 8-4.5 2.598V5.402L11 8Z'
    />
  </svg>
);

export const RedoIcon = (props: SVGProps<SVGSVGElement>) => (
  <svg xmlns='http://www.w3.org/2000/svg' width='1em' height='1em' fill='none' viewBox='0 0 20 20' {...props}>
    <path
      stroke='currentColor'
      strokeLinecap='round'
      strokeLinejoin='round'
      strokeWidth={1.2}
      d='M4.168 8.333h8.333a3.333 3.333 0 0 1 3.334 3.334v0A3.333 3.333 0 0 1 12.5 15h-1.666'
    />
    <path
      stroke='currentColor'
      strokeLinecap='round'
      strokeLinejoin='round'
      strokeWidth={1.2}
      d='M7.501 11.666 4.168 8.333 7.501 5'
    />
  </svg>
);

export const ArrowToLeftIcon = (props: SVGProps<SVGSVGElement>) => (
  <svg xmlns='http://www.w3.org/2000/svg' width='1em' height='1em' fill='none' viewBox='0 0 16 16' {...props}>
    <path
      stroke='currentColor'
      strokeLinecap='round'
      strokeLinejoin='round'
      strokeWidth={1.2}
      d='M14 8H4.667M8.667 4l-4 4 3.943 4M2 4v8'
    />
  </svg>
);

export const CheckListAltIcon = (props: SVGProps<SVGSVGElement>) => (
  <svg xmlns='http://www.w3.org/2000/svg' width='1em' height='1em' fill='none' viewBox='0 0 16 16' {...props}>
    <path
      stroke='currentColor'
      strokeLinecap='round'
      strokeLinejoin='round'
      strokeWidth={1.2}
      d='m2.667 4.667 1.855 2 3.811-4.334M13.333 6h-4M13.333 9.333H2.667M13.333 12.667H2.667'
    />
  </svg>
);

export const Spinner = ({ className, ...props }: SVGProps<SVGSVGElement>) => (
  <svg
    xmlns='http://www.w3.org/2000/svg'
    width='1em'
    height='1em'
    fill='none'
    viewBox='0 0 60 60'
    className={twMerge(tw`animate-spin`, className)}
    {...props}
  >
    <clipPath id='spinner'>
      <path d='M55 30c0 13.807-11.193 25-25 25S5 43.807 5 30 16.193 5 30 5s25 11.193 25 25Zm-41.25 0c0 8.975 7.275 16.25 16.25 16.25S46.25 38.975 46.25 30 38.975 13.75 30 13.75 13.75 21.025 13.75 30Z' />
    </clipPath>
    <foreignObject x='0' y='0' width='100%' height='100%' clipPath='url(#spinner)'>
      <div className={tw`size-full rounded-full`} style={{ backgroundImage: 'conic-gradient(#E2E8F0, #64748B)' }} />
    </foreignObject>
  </svg>
);

export const CheckIcon = (props: SVGProps<SVGSVGElement>) => (
  <svg xmlns='http://www.w3.org/2000/svg' width='1em' height='1em' fill='none' viewBox='0 0 16 16' {...props}>
    <path
      stroke='currentColor'
      strokeLinecap='round'
      strokeLinejoin='round'
      strokeWidth={1.2}
      d='M4.667 8.333 7 10.667l5-5.334'
    />
  </svg>
);
