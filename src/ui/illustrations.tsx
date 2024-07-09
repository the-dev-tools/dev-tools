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

export const Collection = (props: React.ComponentPropsWithoutRef<'svg'>) => (
  <svg width='218' height='130' viewBox='0 0 218 130' fill='none' xmlns='http://www.w3.org/2000/svg' {...props}>
    <g filter='url(#filter0_d_129_9310)'>
      <path
        d='M23.4766 47.2991C23.4766 41.901 27.8526 37.525 33.2506 37.525H184.749C190.147 37.525 194.523 41.901 194.523 47.2991V115.718C194.523 121.116 190.147 125.492 184.749 125.492H33.2506C27.8526 125.492 23.4766 121.116 23.4766 115.718V47.2991Z'
        fill='white'
        shapeRendering='crispEdges'
      />
      <path
        d='M23.8838 47.2991C23.8838 42.1259 28.0775 37.9322 33.2506 37.9322H184.749C189.922 37.9322 194.116 42.1259 194.116 47.2991V115.718C194.116 120.891 189.922 125.084 184.749 125.084H33.2506C28.0775 125.084 23.8838 120.891 23.8838 115.718V47.2991Z'
        stroke='#C6C8CA'
        strokeWidth='0.814506'
        shapeRendering='crispEdges'
      />
      <rect x='36.5088' y='44.8556' width='14.6611' height='14.6611' rx='7.33056' fill='#C8CED7' />
      <rect x='59.3149' y='44.041' width='53.7574' height='4.88704' rx='2.44352' fill='#C8CED7' />
      <rect x='59.3149' y='55.4441' width='26.0642' height='4.88704' rx='2.44352' fill='#C6C8CA' />
      <mask id='path-6-inside-1_129_9310' fill='white'>
        <path d='M23.4766 66.8472H194.523V96.1694H23.4766V66.8472Z' />
      </mask>
      <path d='M23.4766 66.8472H194.523V96.1694H23.4766V66.8472Z' fill='#4F46E5' />
      <path
        d='M193.708 66.8472V96.1694H195.337V66.8472H193.708ZM24.2911 96.1694V66.8472H22.6621V96.1694H24.2911Z'
        fill='#C6C8CA'
        mask='url(#path-6-inside-1_129_9310)'
      />
      <rect x='36.5088' y='74.1778' width='14.6611' height='14.6611' rx='7.33056' fill='white' />
      <rect x='59.3149' y='73.3633' width='53.7574' height='4.88704' rx='2.44352' fill='white' />
      <rect x='59.3149' y='84.7664' width='26.0642' height='4.88704' rx='2.44352' fill='#9EB5FD' />
      <rect x='36.5088' y='103.5' width='14.6611' height='14.6611' rx='7.33056' fill='#C8CED7' />
      <rect x='59.3149' y='102.686' width='53.7574' height='4.88704' rx='2.44352' fill='#C8CED7' />
      <rect x='59.3149' y='114.089' width='26.0642' height='4.88704' rx='2.44352' fill='#C6C8CA' />
    </g>
    <g filter='url(#filter1_d_129_9310)'>
      <path
        d='M18.9756 40.4986C18.9756 34.8164 23.5819 30.2101 29.2641 30.2101H188.736C194.418 30.2101 199.024 34.8164 199.024 40.4986V112.518C199.024 118.2 194.418 122.807 188.736 122.807H29.2641C23.5819 122.807 18.9756 118.2 18.9756 112.518V40.4986Z'
        fill='white'
        shapeRendering='crispEdges'
      />
      <path
        d='M19.4043 40.4986C19.4043 35.0532 23.8187 30.6388 29.2641 30.6388H188.736C194.181 30.6388 198.596 35.0532 198.596 40.4986V112.518C198.596 117.964 194.181 122.378 188.736 122.378H29.2641C23.8187 122.378 19.4043 117.964 19.4043 112.518V40.4986Z'
        stroke='#C6C8CA'
        strokeWidth='0.857375'
        shapeRendering='crispEdges'
      />
      <rect x='32.6934' y='37.9265' width='15.4328' height='15.4328' rx='7.71638' fill='#C8CED7' />
      <rect x='56.7002' y='37.0691' width='56.5868' height='5.14425' rx='2.57212' fill='#C8CED7' />
      <rect x='56.7002' y='49.0723' width='27.436' height='5.14425' rx='2.57212' fill='#C6C8CA' />
      <mask id='path-19-inside-2_129_9310' fill='white'>
        <path d='M18.9756 61.0756H199.024V91.9411H18.9756V61.0756Z' />
      </mask>
      <path d='M18.9756 61.0756H199.024V91.9411H18.9756V61.0756Z' fill='#4F46E5' />
      <path
        d='M198.167 61.0756V91.9411H199.882V61.0756H198.167ZM19.833 91.9411V61.0756H18.1182V91.9411H19.833Z'
        fill='#C6C8CA'
        mask='url(#path-19-inside-2_129_9310)'
      />
      <rect x='32.6934' y='68.7919' width='15.4328' height='15.4328' rx='7.71638' fill='white' />
      <rect x='56.7002' y='67.9346' width='56.5868' height='5.14425' rx='2.57212' fill='white' />
      <rect x='56.7002' y='79.9378' width='27.436' height='5.14425' rx='2.57212' fill='#9EB5FD' />
      <rect x='32.6934' y='99.6574' width='15.4328' height='15.4328' rx='7.71638' fill='#C8CED7' />
      <rect x='56.7002' y='98.8001' width='56.5868' height='5.14425' rx='2.57212' fill='#C8CED7' />
      <rect x='56.7002' y='110.803' width='27.436' height='5.14425' rx='2.57212' fill='#C6C8CA' />
    </g>
    <g filter='url(#filter2_d_129_9310)'>
      <path
        d='M14.2373 32.6033C14.2373 26.6221 19.0861 21.7733 25.0673 21.7733H192.932C198.914 21.7733 203.762 26.6221 203.762 32.6033V108.413C203.762 114.395 198.914 119.243 192.932 119.243H25.0673C19.0861 119.243 14.2373 114.395 14.2373 108.413V32.6033Z'
        fill='white'
        shapeRendering='crispEdges'
      />
      <path
        d='M14.6886 32.6033C14.6886 26.8713 19.3353 22.2246 25.0673 22.2246H192.932C198.664 22.2246 203.311 26.8713 203.311 32.6033V108.413C203.311 114.145 198.664 118.792 192.932 118.792H25.0673C19.3353 118.792 14.6886 114.145 14.6886 108.413V32.6033Z'
        stroke='#C6C8CA'
        strokeWidth='0.9025'
        shapeRendering='crispEdges'
      />
      <rect x='28.6777' y='29.8958' width='16.245' height='16.245' rx='8.1225' fill='#C8CED7' />
      <rect x='53.9473' y='28.9933' width='59.565' height='5.415' rx='2.7075' fill='#C8CED7' />
      <rect x='53.9473' y='41.6283' width='28.88' height='5.415' rx='2.7075' fill='#C6C8CA' />
      <mask id='path-32-inside-3_129_9310' fill='white'>
        <path d='M14.2373 54.2633H203.762V86.7533H14.2373V54.2633Z' />
      </mask>
      <path d='M14.2373 54.2633H203.762V86.7533H14.2373V54.2633Z' fill='#4F46E5' />
      <path
        d='M202.86 54.2633V86.7533H204.665V54.2633H202.86ZM15.1398 86.7533V54.2633H13.3348V86.7533H15.1398Z'
        fill='#C6C8CA'
        mask='url(#path-32-inside-3_129_9310)'
      />
      <rect x='28.6772' y='62.3858' width='16.245' height='16.245' rx='8.1225' fill='white' />
      <rect x='53.9473' y='61.4833' width='59.565' height='5.415' rx='2.7075' fill='white' />
      <rect x='53.9473' y='74.1183' width='28.88' height='5.415' rx='2.7075' fill='#9EB5FD' />
      <rect x='28.6772' y='94.8758' width='16.245' height='16.245' rx='8.1225' fill='#C8CED7' />
      <rect x='53.9473' y='93.9733' width='59.565' height='5.415' rx='2.7075' fill='#C8CED7' />
      <rect x='53.9473' y='106.608' width='28.88' height='5.415' rx='2.7075' fill='#C6C8CA' />
    </g>
    <g filter='url(#filter3_d_129_9310)'>
      <path
        d='M9.25 24.6083C9.25 18.3123 14.354 13.2083 20.65 13.2083H197.35C203.646 13.2083 208.75 18.3123 208.75 24.6083V104.408C208.75 110.704 203.646 115.808 197.35 115.808H20.65C14.354 115.808 9.25 110.704 9.25 104.408V24.6083Z'
        fill='white'
        shapeRendering='crispEdges'
      />
      <path
        d='M9.725 24.6083C9.725 18.5746 14.6163 13.6833 20.65 13.6833H197.35C203.384 13.6833 208.275 18.5746 208.275 24.6083V104.408C208.275 110.442 203.384 115.333 197.35 115.333H20.65C14.6163 115.333 9.725 110.442 9.725 104.408V24.6083Z'
        stroke='#C6C8CA'
        strokeWidth='0.95'
        shapeRendering='crispEdges'
      />
      <rect x='24.4502' y='21.7583' width='17.1' height='17.1' rx='8.55' fill='#C8CED7' />
      <rect x='51.0498' y='20.8083' width='62.7' height='5.7' rx='2.85' fill='#C8CED7' />
      <rect x='51.0498' y='34.1083' width='30.4' height='5.7' rx='2.85' fill='#C6C8CA' />
      <mask id='path-45-inside-4_129_9310' fill='white'>
        <path d='M9.25 47.4083H208.75V81.6083H9.25V47.4083Z' />
      </mask>
      <path d='M9.25 47.4083H208.75V81.6083H9.25V47.4083Z' fill='#4F46E5' />
      <path
        d='M207.8 47.4083V81.6083H209.7V47.4083H207.8ZM10.2 81.6083V47.4083H8.3V81.6083H10.2Z'
        fill='#C6C8CA'
        mask='url(#path-45-inside-4_129_9310)'
      />
      <rect x='24.4502' y='55.9583' width='17.1' height='17.1' rx='8.55' fill='white' />
      <rect x='51.0498' y='55.0083' width='62.7' height='5.7' rx='2.85' fill='white' />
      <rect x='51.0498' y='68.3083' width='30.4' height='5.7' rx='2.85' fill='#9EB5FD' />
      <rect x='24.4502' y='90.1583' width='17.1' height='17.1' rx='8.55' fill='#C8CED7' />
      <rect x='51.0498' y='89.2083' width='62.7' height='5.7' rx='2.85' fill='#C8CED7' />
      <rect x='51.0498' y='102.508' width='30.4' height='5.7' rx='2.85' fill='#C6C8CA' />
    </g>
    <g filter='url(#filter4_d_129_9310)'>
      <path
        d='M4 16.5083C4 9.88091 9.37258 4.50833 16 4.50833H202C208.627 4.50833 214 9.88091 214 16.5083V100.508C214 107.136 208.627 112.508 202 112.508H16C9.37258 112.508 4 107.136 4 100.508V16.5083Z'
        fill='white'
        shapeRendering='crispEdges'
      />
      <path
        d='M4.5 16.5083C4.5 10.1571 9.64873 5.00833 16 5.00833H202C208.351 5.00833 213.5 10.1571 213.5 16.5083V100.508C213.5 106.86 208.351 112.008 202 112.008H16C9.64873 112.008 4.5 106.86 4.5 100.508V16.5083Z'
        stroke='#C6C8CA'
        shapeRendering='crispEdges'
      />
      <rect x='20' y='13.5083' width='18' height='18' rx='9' fill='#C8CED7' />
      <rect x='48' y='12.5083' width='66' height='6' rx='3' fill='#C8CED7' />
      <rect x='48' y='26.5083' width='32' height='6' rx='3' fill='#C6C8CA' />
      <mask id='path-58-inside-5_129_9310' fill='white'>
        <path d='M4 40.5083H214V76.5083H4V40.5083Z' />
      </mask>
      <path d='M4 40.5083H214V76.5083H4V40.5083Z' fill='#4F46E5' />
      <path
        d='M213 40.5083V76.5083H215V40.5083H213ZM5 76.5083V40.5083H3V76.5083H5Z'
        fill='#C6C8CA'
        mask='url(#path-58-inside-5_129_9310)'
      />
      <rect x='20' y='49.5083' width='18' height='18' rx='9' fill='white' />
      <rect x='48' y='48.5083' width='66' height='6' rx='3' fill='white' />
      <rect x='48' y='62.5083' width='32' height='6' rx='3' fill='#9EB5FD' />
      <rect x='20' y='85.5083' width='18' height='18' rx='9' fill='#C8CED7' />
      <rect x='48' y='84.5083' width='66' height='6' rx='3' fill='#C8CED7' />
      <rect x='48' y='98.5083' width='32' height='6' rx='3' fill='#C6C8CA' />
    </g>
    <defs>
      <filter
        id='filter0_d_129_9310'
        x='19.8666'
        y='33.915'
        width='178.266'
        height='95.1867'
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
        <feOffset />
        <feGaussianBlur stdDeviation='1.805' />
        <feComposite in2='hardAlpha' operator='out' />
        <feColorMatrix type='matrix' values='0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0.25 0' />
        <feBlend mode='normal' in2='BackgroundImageFix' result='effect1_dropShadow_129_9310' />
        <feBlend mode='normal' in='SourceGraphic' in2='effect1_dropShadow_129_9310' result='shape' />
      </filter>
      <filter
        id='filter1_d_129_9310'
        x='15.1756'
        y='26.4101'
        width='187.649'
        height='100.197'
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
        <feOffset />
        <feGaussianBlur stdDeviation='1.9' />
        <feComposite in2='hardAlpha' operator='out' />
        <feColorMatrix type='matrix' values='0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0.25 0' />
        <feBlend mode='normal' in2='BackgroundImageFix' result='effect1_dropShadow_129_9310' />
        <feBlend mode='normal' in='SourceGraphic' in2='effect1_dropShadow_129_9310' result='shape' />
      </filter>
      <filter
        id='filter2_d_129_9310'
        x='10.2373'
        y='17.7733'
        width='197.525'
        height='105.47'
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
        <feOffset />
        <feGaussianBlur stdDeviation='2' />
        <feComposite in2='hardAlpha' operator='out' />
        <feColorMatrix type='matrix' values='0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0.25 0' />
        <feBlend mode='normal' in2='BackgroundImageFix' result='effect1_dropShadow_129_9310' />
        <feBlend mode='normal' in='SourceGraphic' in2='effect1_dropShadow_129_9310' result='shape' />
      </filter>
      <filter
        id='filter3_d_129_9310'
        x='5.25'
        y='9.20833'
        width='207.5'
        height='110.6'
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
        <feOffset />
        <feGaussianBlur stdDeviation='2' />
        <feComposite in2='hardAlpha' operator='out' />
        <feColorMatrix type='matrix' values='0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0.25 0' />
        <feBlend mode='normal' in2='BackgroundImageFix' result='effect1_dropShadow_129_9310' />
        <feBlend mode='normal' in='SourceGraphic' in2='effect1_dropShadow_129_9310' result='shape' />
      </filter>
      <filter
        id='filter4_d_129_9310'
        x='0'
        y='0.508331'
        width='218'
        height='116'
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
        <feOffset />
        <feGaussianBlur stdDeviation='2' />
        <feComposite in2='hardAlpha' operator='out' />
        <feColorMatrix type='matrix' values='0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0.25 0' />
        <feBlend mode='normal' in2='BackgroundImageFix' result='effect1_dropShadow_129_9310' />
        <feBlend mode='normal' in='SourceGraphic' in2='effect1_dropShadow_129_9310' result='shape' />
      </filter>
    </defs>
  </svg>
);
