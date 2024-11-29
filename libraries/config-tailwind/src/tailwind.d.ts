import 'tailwindcss/types/config';

declare module 'tailwindcss/types/config' {
  export interface ThemeConfig {
    size: ThemeConfig['spacing'];
  }
}
