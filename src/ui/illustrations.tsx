import { twJoin } from 'tailwind-merge';

export const Logo = (props: React.ComponentPropsWithoutRef<'svg'>) => (
  <svg width='155' height='100' viewBox='0 0 155 100' fill='none' xmlns='http://www.w3.org/2000/svg' {...props}>
    <path
      d='M64.2454 99.2922L89.5271 21.8757C89.9806 18.9968 91.7275 16.4252 94.3051 14.9454C97.3362 13.2044 101.089 13.2072 104.111 14.9385L150.099 41.3308C153.121 43.067 155 46.2974 155 49.765C155 53.2326 153.121 56.4658 150.099 58.2019L116.674 77.3827C109.738 81.3631 100.869 79.001 96.864 72.108L135.794 49.7657L102 30.3747L82.5183 90.0316C80.0454 97.6042 71.8645 101.749 64.2454 99.2922Z'
      fill='url(#paint0_linear_140_631)'
    />
    <path
      d='M55.7852 86.3574C54.0903 86.3574 52.3955 85.9232 50.8844 85.0555L4.90072 58.6674C1.87929 56.9387 0 53.7076 0 50.238C0 46.7684 1.87444 43.5324 4.90072 41.7962L38.3258 22.6154C45.2618 18.6351 54.1312 20.9971 58.136 27.8902L19.2055 50.2325L52.9947 69.6214L72.4769 9.96932C74.9498 2.39736 83.1306 -1.74909 90.7497 0.707353L65.468 78.1217C65.0145 81.0028 63.2677 83.5764 60.6852 85.0568C59.1741 85.9225 57.48 86.3574 55.7852 86.3574Z'
      fill='url(#paint1_linear_140_631)'
    />
    <defs>
      <linearGradient
        id='paint0_linear_140_631'
        x1='155.949'
        y1='50'
        x2='1.93496'
        y2='43.0949'
        gradientUnits='userSpaceOnUse'
      >
        <stop stopColor='#6366F1' />
        <stop offset='1' stopColor='#1F2172' />
      </linearGradient>
      <linearGradient
        id='paint1_linear_140_631'
        x1='155.949'
        y1='50'
        x2='1.93496'
        y2='43.0949'
        gradientUnits='userSpaceOnUse'
      >
        <stop stopColor='#6366F1' />
        <stop offset='1' stopColor='#1F2172' />
      </linearGradient>
    </defs>
  </svg>
);

export const IntroIcon = ({ className, ...props }: React.ComponentPropsWithoutRef<'div'>) => (
  <div className={twJoin('relative flex items-center justify-center', className)} {...props}>
    <svg width='78' height='79' viewBox='0 0 78 79' fill='none' xmlns='http://www.w3.org/2000/svg'>
      <g filter='url(#filter0_d_129_8606)'>
        <rect
          x='3.75'
          y='2.75'
          width='70.5'
          height='70.5'
          rx='35.25'
          stroke='#EAECF0'
          strokeWidth='1.5'
          shapeRendering='crispEdges'
        />
        <circle cx='39' cy='38' r='12' fill='#FECACA' />
        <circle cx='39.0005' cy='38.0001' r='8' fill='#EF4444' />
      </g>
      <defs>
        <filter
          id='filter0_d_129_8606'
          x='0'
          y='0.5'
          width='78'
          height='78'
          filterUnits='userSpaceOnUse'
          colorInterpolationFilters='sRGB'
        >
          <feFlood floodOpacity='0' result='BackgroundImageFix' />
          <feColorMatrix
            in='SourceAlpha'
            type='matrix'
            values='0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 127 0'
            result='hardAlpha'
          />
          <feOffset dy='1.5' />
          <feGaussianBlur stdDeviation='1.5' />
          <feComposite in2='hardAlpha' operator='out' />
          <feColorMatrix type='matrix' values='0 0 0 0 0.0627451 0 0 0 0 0.0941176 0 0 0 0 0.156863 0 0 0 0.05 0' />
          <feBlend mode='normal' in2='BackgroundImageFix' result='effect1_dropShadow_129_8606' />
          <feBlend mode='normal' in='SourceGraphic' in2='effect1_dropShadow_129_8606' result='shape' />
        </filter>
      </defs>
    </svg>
    <div className='absolute inset-0 -z-10'>
      <svg
        className='absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2'
        width='720'
        height='720'
        viewBox='0 0 720 720'
        fill='none'
        xmlns='http://www.w3.org/2000/svg'
      >
        <mask
          id='mask0_173_747'
          style={{ maskType: 'alpha' }}
          maskUnits='userSpaceOnUse'
          x='0'
          y='0'
          width='720'
          height='720'
        >
          <rect width='720' height='720' fill='url(#paint0_radial_173_747)' />
        </mask>
        <g mask='url(#mask0_173_747)'>
          <circle cx='360' cy='360' r='71.25' stroke='#EAECF0' strokeWidth='1.5' />
          <circle cx='360' cy='360' r='119.25' stroke='#EAECF0' strokeWidth='1.5' />
          <circle cx='360' cy='360' r='167.25' stroke='#EAECF0' strokeWidth='1.5' />
          <circle cx='360' cy='360' r='215.25' stroke='#EAECF0' strokeWidth='1.5' />
          <circle cx='360' cy='360' r='215.25' stroke='#EAECF0' strokeWidth='1.5' />
          <circle cx='360' cy='360' r='263.25' stroke='#EAECF0' strokeWidth='1.5' />
          <circle cx='360' cy='360' r='311.25' stroke='#EAECF0' strokeWidth='1.5' />
          <circle cx='360' cy='360' r='359.25' stroke='#EAECF0' strokeWidth='1.5' />
        </g>
        <defs>
          <radialGradient
            id='paint0_radial_173_747'
            cx='0'
            cy='0'
            r='1'
            gradientUnits='userSpaceOnUse'
            gradientTransform='translate(360 360) rotate(90) scale(219.259 219.259)'
          >
            <stop />
            <stop offset='1' stopOpacity='0' />
          </radialGradient>
        </defs>
      </svg>
    </div>
  </div>
);
