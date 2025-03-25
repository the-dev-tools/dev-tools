import { twJoin } from 'tailwind-merge';

export const Logo = (props: React.ComponentPropsWithoutRef<'svg'>) => (
  <svg fill='none' height='1em' viewBox='0 0 24 20' width='1em' xmlns='http://www.w3.org/2000/svg' {...props}>
    <path
      d='M14.817 0h-3.238l8.24 10.008-6.294 7.645H10.97l6.294-7.645L9.027 0H0l8.238 10.008-5.69 6.91V6.157L.049 3.122V20h3.197l8.228-9.992L5.18 2.363h2.555l6.294 7.645L5.8 20h9.026l8.228-9.992L14.817 0Z'
      fill='url(#a)'
    />
    <defs>
      <linearGradient gradientUnits='userSpaceOnUse' id='a' x1={-9.312} x2={21.745} y1={10.001} y2={10.001}>
        <stop offset={0.145} stopColor='#A555F1' />
        <stop offset={0.732} stopColor='#8BA6FF' />
        <stop offset={1} stopColor='#00C7FF' />
      </linearGradient>
    </defs>
  </svg>
);

export const IntroIcon = ({ className, ...props }: React.ComponentPropsWithoutRef<'div'>) => (
  <div className={twJoin('relative flex items-center justify-center', className)} {...props}>
    <svg fill='none' height='79' viewBox='0 0 78 79' width='78' xmlns='http://www.w3.org/2000/svg'>
      <g filter='url(#filter0_d_129_8606)'>
        <rect
          height='70.5'
          rx='35.25'
          shapeRendering='crispEdges'
          stroke='#EAECF0'
          strokeWidth='1.5'
          width='70.5'
          x='3.75'
          y='2.75'
        />
        <circle cx='39' cy='38' fill='#FECACA' r='12' />
        <circle cx='39.0005' cy='38.0001' fill='#EF4444' r='8' />
      </g>
      <defs>
        <filter
          colorInterpolationFilters='sRGB'
          filterUnits='userSpaceOnUse'
          height='78'
          id='filter0_d_129_8606'
          width='78'
          x='0'
          y='0.5'
        >
          <feFlood floodOpacity='0' result='BackgroundImageFix' />
          <feColorMatrix
            in='SourceAlpha'
            result='hardAlpha'
            type='matrix'
            values='0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 127 0'
          />
          <feOffset dy='1.5' />
          <feGaussianBlur stdDeviation='1.5' />
          <feComposite in2='hardAlpha' operator='out' />
          <feColorMatrix type='matrix' values='0 0 0 0 0.0627451 0 0 0 0 0.0941176 0 0 0 0 0.156863 0 0 0 0.05 0' />
          <feBlend in2='BackgroundImageFix' mode='normal' result='effect1_dropShadow_129_8606' />
          <feBlend in='SourceGraphic' in2='effect1_dropShadow_129_8606' mode='normal' result='shape' />
        </filter>
      </defs>
    </svg>
    <div className='absolute inset-0 -z-10'>
      <svg
        className='absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2'
        fill='none'
        height='720'
        viewBox='0 0 720 720'
        width='720'
        xmlns='http://www.w3.org/2000/svg'
      >
        <mask
          height='720'
          id='mask0_173_747'
          maskUnits='userSpaceOnUse'
          style={{ maskType: 'alpha' }}
          width='720'
          x='0'
          y='0'
        >
          <rect fill='url(#paint0_radial_173_747)' height='720' width='720' />
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
            cx='0'
            cy='0'
            gradientTransform='translate(360 360) rotate(90) scale(219.259 219.259)'
            gradientUnits='userSpaceOnUse'
            id='paint0_radial_173_747'
            r='1'
          >
            <stop />
            <stop offset='1' stopOpacity='0' />
          </radialGradient>
        </defs>
      </svg>
    </div>
  </div>
);

export const EmptyCollectionIllustration = (props: React.ComponentPropsWithoutRef<'svg'>) => (
  <svg fill='none' height='130' viewBox='0 0 218 130' width='218' xmlns='http://www.w3.org/2000/svg' {...props}>
    <g filter='url(#filter0_d_129_9310)'>
      <path
        d='M23.4766 47.2991C23.4766 41.901 27.8526 37.525 33.2506 37.525H184.749C190.147 37.525 194.523 41.901 194.523 47.2991V115.718C194.523 121.116 190.147 125.492 184.749 125.492H33.2506C27.8526 125.492 23.4766 121.116 23.4766 115.718V47.2991Z'
        fill='white'
        shapeRendering='crispEdges'
      />
      <path
        d='M23.8838 47.2991C23.8838 42.1259 28.0775 37.9322 33.2506 37.9322H184.749C189.922 37.9322 194.116 42.1259 194.116 47.2991V115.718C194.116 120.891 189.922 125.084 184.749 125.084H33.2506C28.0775 125.084 23.8838 120.891 23.8838 115.718V47.2991Z'
        shapeRendering='crispEdges'
        stroke='#C6C8CA'
        strokeWidth='0.814506'
      />
      <rect fill='#C8CED7' height='14.6611' rx='7.33056' width='14.6611' x='36.5088' y='44.8556' />
      <rect fill='#C8CED7' height='4.88704' rx='2.44352' width='53.7574' x='59.3149' y='44.041' />
      <rect fill='#C6C8CA' height='4.88704' rx='2.44352' width='26.0642' x='59.3149' y='55.4441' />
      <mask fill='white' id='path-6-inside-1_129_9310'>
        <path d='M23.4766 66.8472H194.523V96.1694H23.4766V66.8472Z' />
      </mask>
      <path d='M23.4766 66.8472H194.523V96.1694H23.4766V66.8472Z' fill='#4F46E5' />
      <path
        d='M193.708 66.8472V96.1694H195.337V66.8472H193.708ZM24.2911 96.1694V66.8472H22.6621V96.1694H24.2911Z'
        fill='#C6C8CA'
        mask='url(#path-6-inside-1_129_9310)'
      />
      <rect fill='white' height='14.6611' rx='7.33056' width='14.6611' x='36.5088' y='74.1778' />
      <rect fill='white' height='4.88704' rx='2.44352' width='53.7574' x='59.3149' y='73.3633' />
      <rect fill='#9EB5FD' height='4.88704' rx='2.44352' width='26.0642' x='59.3149' y='84.7664' />
      <rect fill='#C8CED7' height='14.6611' rx='7.33056' width='14.6611' x='36.5088' y='103.5' />
      <rect fill='#C8CED7' height='4.88704' rx='2.44352' width='53.7574' x='59.3149' y='102.686' />
      <rect fill='#C6C8CA' height='4.88704' rx='2.44352' width='26.0642' x='59.3149' y='114.089' />
    </g>
    <g filter='url(#filter1_d_129_9310)'>
      <path
        d='M18.9756 40.4986C18.9756 34.8164 23.5819 30.2101 29.2641 30.2101H188.736C194.418 30.2101 199.024 34.8164 199.024 40.4986V112.518C199.024 118.2 194.418 122.807 188.736 122.807H29.2641C23.5819 122.807 18.9756 118.2 18.9756 112.518V40.4986Z'
        fill='white'
        shapeRendering='crispEdges'
      />
      <path
        d='M19.4043 40.4986C19.4043 35.0532 23.8187 30.6388 29.2641 30.6388H188.736C194.181 30.6388 198.596 35.0532 198.596 40.4986V112.518C198.596 117.964 194.181 122.378 188.736 122.378H29.2641C23.8187 122.378 19.4043 117.964 19.4043 112.518V40.4986Z'
        shapeRendering='crispEdges'
        stroke='#C6C8CA'
        strokeWidth='0.857375'
      />
      <rect fill='#C8CED7' height='15.4328' rx='7.71638' width='15.4328' x='32.6934' y='37.9265' />
      <rect fill='#C8CED7' height='5.14425' rx='2.57212' width='56.5868' x='56.7002' y='37.0691' />
      <rect fill='#C6C8CA' height='5.14425' rx='2.57212' width='27.436' x='56.7002' y='49.0723' />
      <mask fill='white' id='path-19-inside-2_129_9310'>
        <path d='M18.9756 61.0756H199.024V91.9411H18.9756V61.0756Z' />
      </mask>
      <path d='M18.9756 61.0756H199.024V91.9411H18.9756V61.0756Z' fill='#4F46E5' />
      <path
        d='M198.167 61.0756V91.9411H199.882V61.0756H198.167ZM19.833 91.9411V61.0756H18.1182V91.9411H19.833Z'
        fill='#C6C8CA'
        mask='url(#path-19-inside-2_129_9310)'
      />
      <rect fill='white' height='15.4328' rx='7.71638' width='15.4328' x='32.6934' y='68.7919' />
      <rect fill='white' height='5.14425' rx='2.57212' width='56.5868' x='56.7002' y='67.9346' />
      <rect fill='#9EB5FD' height='5.14425' rx='2.57212' width='27.436' x='56.7002' y='79.9378' />
      <rect fill='#C8CED7' height='15.4328' rx='7.71638' width='15.4328' x='32.6934' y='99.6574' />
      <rect fill='#C8CED7' height='5.14425' rx='2.57212' width='56.5868' x='56.7002' y='98.8001' />
      <rect fill='#C6C8CA' height='5.14425' rx='2.57212' width='27.436' x='56.7002' y='110.803' />
    </g>
    <g filter='url(#filter2_d_129_9310)'>
      <path
        d='M14.2373 32.6033C14.2373 26.6221 19.0861 21.7733 25.0673 21.7733H192.932C198.914 21.7733 203.762 26.6221 203.762 32.6033V108.413C203.762 114.395 198.914 119.243 192.932 119.243H25.0673C19.0861 119.243 14.2373 114.395 14.2373 108.413V32.6033Z'
        fill='white'
        shapeRendering='crispEdges'
      />
      <path
        d='M14.6886 32.6033C14.6886 26.8713 19.3353 22.2246 25.0673 22.2246H192.932C198.664 22.2246 203.311 26.8713 203.311 32.6033V108.413C203.311 114.145 198.664 118.792 192.932 118.792H25.0673C19.3353 118.792 14.6886 114.145 14.6886 108.413V32.6033Z'
        shapeRendering='crispEdges'
        stroke='#C6C8CA'
        strokeWidth='0.9025'
      />
      <rect fill='#C8CED7' height='16.245' rx='8.1225' width='16.245' x='28.6777' y='29.8958' />
      <rect fill='#C8CED7' height='5.415' rx='2.7075' width='59.565' x='53.9473' y='28.9933' />
      <rect fill='#C6C8CA' height='5.415' rx='2.7075' width='28.88' x='53.9473' y='41.6283' />
      <mask fill='white' id='path-32-inside-3_129_9310'>
        <path d='M14.2373 54.2633H203.762V86.7533H14.2373V54.2633Z' />
      </mask>
      <path d='M14.2373 54.2633H203.762V86.7533H14.2373V54.2633Z' fill='#4F46E5' />
      <path
        d='M202.86 54.2633V86.7533H204.665V54.2633H202.86ZM15.1398 86.7533V54.2633H13.3348V86.7533H15.1398Z'
        fill='#C6C8CA'
        mask='url(#path-32-inside-3_129_9310)'
      />
      <rect fill='white' height='16.245' rx='8.1225' width='16.245' x='28.6772' y='62.3858' />
      <rect fill='white' height='5.415' rx='2.7075' width='59.565' x='53.9473' y='61.4833' />
      <rect fill='#9EB5FD' height='5.415' rx='2.7075' width='28.88' x='53.9473' y='74.1183' />
      <rect fill='#C8CED7' height='16.245' rx='8.1225' width='16.245' x='28.6772' y='94.8758' />
      <rect fill='#C8CED7' height='5.415' rx='2.7075' width='59.565' x='53.9473' y='93.9733' />
      <rect fill='#C6C8CA' height='5.415' rx='2.7075' width='28.88' x='53.9473' y='106.608' />
    </g>
    <g filter='url(#filter3_d_129_9310)'>
      <path
        d='M9.25 24.6083C9.25 18.3123 14.354 13.2083 20.65 13.2083H197.35C203.646 13.2083 208.75 18.3123 208.75 24.6083V104.408C208.75 110.704 203.646 115.808 197.35 115.808H20.65C14.354 115.808 9.25 110.704 9.25 104.408V24.6083Z'
        fill='white'
        shapeRendering='crispEdges'
      />
      <path
        d='M9.725 24.6083C9.725 18.5746 14.6163 13.6833 20.65 13.6833H197.35C203.384 13.6833 208.275 18.5746 208.275 24.6083V104.408C208.275 110.442 203.384 115.333 197.35 115.333H20.65C14.6163 115.333 9.725 110.442 9.725 104.408V24.6083Z'
        shapeRendering='crispEdges'
        stroke='#C6C8CA'
        strokeWidth='0.95'
      />
      <rect fill='#C8CED7' height='17.1' rx='8.55' width='17.1' x='24.4502' y='21.7583' />
      <rect fill='#C8CED7' height='5.7' rx='2.85' width='62.7' x='51.0498' y='20.8083' />
      <rect fill='#C6C8CA' height='5.7' rx='2.85' width='30.4' x='51.0498' y='34.1083' />
      <mask fill='white' id='path-45-inside-4_129_9310'>
        <path d='M9.25 47.4083H208.75V81.6083H9.25V47.4083Z' />
      </mask>
      <path d='M9.25 47.4083H208.75V81.6083H9.25V47.4083Z' fill='#4F46E5' />
      <path
        d='M207.8 47.4083V81.6083H209.7V47.4083H207.8ZM10.2 81.6083V47.4083H8.3V81.6083H10.2Z'
        fill='#C6C8CA'
        mask='url(#path-45-inside-4_129_9310)'
      />
      <rect fill='white' height='17.1' rx='8.55' width='17.1' x='24.4502' y='55.9583' />
      <rect fill='white' height='5.7' rx='2.85' width='62.7' x='51.0498' y='55.0083' />
      <rect fill='#9EB5FD' height='5.7' rx='2.85' width='30.4' x='51.0498' y='68.3083' />
      <rect fill='#C8CED7' height='17.1' rx='8.55' width='17.1' x='24.4502' y='90.1583' />
      <rect fill='#C8CED7' height='5.7' rx='2.85' width='62.7' x='51.0498' y='89.2083' />
      <rect fill='#C6C8CA' height='5.7' rx='2.85' width='30.4' x='51.0498' y='102.508' />
    </g>
    <g filter='url(#filter4_d_129_9310)'>
      <path
        d='M4 16.5083C4 9.88091 9.37258 4.50833 16 4.50833H202C208.627 4.50833 214 9.88091 214 16.5083V100.508C214 107.136 208.627 112.508 202 112.508H16C9.37258 112.508 4 107.136 4 100.508V16.5083Z'
        fill='white'
        shapeRendering='crispEdges'
      />
      <path
        d='M4.5 16.5083C4.5 10.1571 9.64873 5.00833 16 5.00833H202C208.351 5.00833 213.5 10.1571 213.5 16.5083V100.508C213.5 106.86 208.351 112.008 202 112.008H16C9.64873 112.008 4.5 106.86 4.5 100.508V16.5083Z'
        shapeRendering='crispEdges'
        stroke='#C6C8CA'
      />
      <rect fill='#C8CED7' height='18' rx='9' width='18' x='20' y='13.5083' />
      <rect fill='#C8CED7' height='6' rx='3' width='66' x='48' y='12.5083' />
      <rect fill='#C6C8CA' height='6' rx='3' width='32' x='48' y='26.5083' />
      <mask fill='white' id='path-58-inside-5_129_9310'>
        <path d='M4 40.5083H214V76.5083H4V40.5083Z' />
      </mask>
      <path d='M4 40.5083H214V76.5083H4V40.5083Z' fill='#4F46E5' />
      <path
        d='M213 40.5083V76.5083H215V40.5083H213ZM5 76.5083V40.5083H3V76.5083H5Z'
        fill='#C6C8CA'
        mask='url(#path-58-inside-5_129_9310)'
      />
      <rect fill='white' height='18' rx='9' width='18' x='20' y='49.5083' />
      <rect fill='white' height='6' rx='3' width='66' x='48' y='48.5083' />
      <rect fill='#9EB5FD' height='6' rx='3' width='32' x='48' y='62.5083' />
      <rect fill='#C8CED7' height='18' rx='9' width='18' x='20' y='85.5083' />
      <rect fill='#C8CED7' height='6' rx='3' width='66' x='48' y='84.5083' />
      <rect fill='#C6C8CA' height='6' rx='3' width='32' x='48' y='98.5083' />
    </g>
    <defs>
      <filter
        colorInterpolationFilters='sRGB'
        filterUnits='userSpaceOnUse'
        height='95.1867'
        id='filter0_d_129_9310'
        width='178.266'
        x='19.8666'
        y='33.915'
      >
        <feFlood floodOpacity='0' result='BackgroundImageFix' />
        <feColorMatrix
          in='SourceAlpha'
          result='hardAlpha'
          type='matrix'
          values='0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 127 0'
        />
        <feOffset />
        <feGaussianBlur stdDeviation='1.805' />
        <feComposite in2='hardAlpha' operator='out' />
        <feColorMatrix type='matrix' values='0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0.25 0' />
        <feBlend in2='BackgroundImageFix' mode='normal' result='effect1_dropShadow_129_9310' />
        <feBlend in='SourceGraphic' in2='effect1_dropShadow_129_9310' mode='normal' result='shape' />
      </filter>
      <filter
        colorInterpolationFilters='sRGB'
        filterUnits='userSpaceOnUse'
        height='100.197'
        id='filter1_d_129_9310'
        width='187.649'
        x='15.1756'
        y='26.4101'
      >
        <feFlood floodOpacity='0' result='BackgroundImageFix' />
        <feColorMatrix
          in='SourceAlpha'
          result='hardAlpha'
          type='matrix'
          values='0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 127 0'
        />
        <feOffset />
        <feGaussianBlur stdDeviation='1.9' />
        <feComposite in2='hardAlpha' operator='out' />
        <feColorMatrix type='matrix' values='0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0.25 0' />
        <feBlend in2='BackgroundImageFix' mode='normal' result='effect1_dropShadow_129_9310' />
        <feBlend in='SourceGraphic' in2='effect1_dropShadow_129_9310' mode='normal' result='shape' />
      </filter>
      <filter
        colorInterpolationFilters='sRGB'
        filterUnits='userSpaceOnUse'
        height='105.47'
        id='filter2_d_129_9310'
        width='197.525'
        x='10.2373'
        y='17.7733'
      >
        <feFlood floodOpacity='0' result='BackgroundImageFix' />
        <feColorMatrix
          in='SourceAlpha'
          result='hardAlpha'
          type='matrix'
          values='0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 127 0'
        />
        <feOffset />
        <feGaussianBlur stdDeviation='2' />
        <feComposite in2='hardAlpha' operator='out' />
        <feColorMatrix type='matrix' values='0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0.25 0' />
        <feBlend in2='BackgroundImageFix' mode='normal' result='effect1_dropShadow_129_9310' />
        <feBlend in='SourceGraphic' in2='effect1_dropShadow_129_9310' mode='normal' result='shape' />
      </filter>
      <filter
        colorInterpolationFilters='sRGB'
        filterUnits='userSpaceOnUse'
        height='110.6'
        id='filter3_d_129_9310'
        width='207.5'
        x='5.25'
        y='9.20833'
      >
        <feFlood floodOpacity='0' result='BackgroundImageFix' />
        <feColorMatrix
          in='SourceAlpha'
          result='hardAlpha'
          type='matrix'
          values='0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 127 0'
        />
        <feOffset />
        <feGaussianBlur stdDeviation='2' />
        <feComposite in2='hardAlpha' operator='out' />
        <feColorMatrix type='matrix' values='0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0.25 0' />
        <feBlend in2='BackgroundImageFix' mode='normal' result='effect1_dropShadow_129_9310' />
        <feBlend in='SourceGraphic' in2='effect1_dropShadow_129_9310' mode='normal' result='shape' />
      </filter>
      <filter
        colorInterpolationFilters='sRGB'
        filterUnits='userSpaceOnUse'
        height='116'
        id='filter4_d_129_9310'
        width='218'
        x='0'
        y='0.508331'
      >
        <feFlood floodOpacity='0' result='BackgroundImageFix' />
        <feColorMatrix
          in='SourceAlpha'
          result='hardAlpha'
          type='matrix'
          values='0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 127 0'
        />
        <feOffset />
        <feGaussianBlur stdDeviation='2' />
        <feComposite in2='hardAlpha' operator='out' />
        <feColorMatrix type='matrix' values='0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0.25 0' />
        <feBlend in2='BackgroundImageFix' mode='normal' result='effect1_dropShadow_129_9310' />
        <feBlend in='SourceGraphic' in2='effect1_dropShadow_129_9310' mode='normal' result='shape' />
      </filter>
    </defs>
  </svg>
);
