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
  <svg xmlns='http://www.w3.org/2000/svg' width={16} height={16} fill='none' {...props}>
    <path stroke='currentColor' strokeWidth={1.2} d='M11.111 4.444H9a1 1 0 0 0-1 1v5.2a1 1 0 0 0 1 1h2.111M5.333 8H8' />
    <rect width={3.556} height={3.556} x={1.778} y={6.222} stroke='currentColor' strokeWidth={1.2} rx={1} />
    <rect width={3.556} height={3.556} x={10.667} y={2.8} stroke='currentColor' strokeWidth={1.2} rx={1} />
    <rect width={3.556} height={3.556} x={10.667} y={10} stroke='currentColor' strokeWidth={1.2} rx={1} />
  </svg>
);
