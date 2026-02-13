import { Option, pipe } from 'effect';
import { createContext, PropsWithChildren, use, useState } from 'react';

type Theme = 'dark' | 'light';

const getStoreTheme = () => localStorage.getItem('theme') as Theme | undefined;
const getSystemTheme = () => (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');

export const setTheme = (theme?: Theme) => {
  const systemTheme = getSystemTheme();
  const storeTheme = theme ?? getStoreTheme() ?? systemTheme;

  if (storeTheme === systemTheme) localStorage.removeItem('theme');
  else localStorage.setItem('theme', storeTheme);

  if (storeTheme === 'dark') document.documentElement.classList.add('dark');
  else document.documentElement.classList.remove('dark');
};

interface ThemeContext {
  theme: Theme;
  toggleTheme: () => void;
}

const ThemeContext = createContext(Option.none<ThemeContext>());

export const useTheme = () => pipe(use(ThemeContext), Option.getOrThrow);

export const ThemeProvider = ({ children }: PropsWithChildren) => {
  const [theme, setThemeState] = useState(getStoreTheme() ?? getSystemTheme());

  const toggleTheme = () => {
    const nextTheme = theme === 'dark' ? 'light' : 'dark';
    setTheme(nextTheme);
    setThemeState(nextTheme);
  };

  return <ThemeContext.Provider value={Option.some({ theme, toggleTheme })}>{children}</ThemeContext.Provider>;
};
