import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { readStoredThemeMode, storeThemeMode, type ThemeMode } from '@/features/mobile/mobile-workspace-utils';

type ThemeContextValue = {
  setThemeMode: (mode: ThemeMode) => void;
  themeMode: ThemeMode;
};

const ThemeContext = createContext<ThemeContextValue | null>(null);

type ThemeProviderProps = {
  children: ReactNode;
};

// ThemeProvider owns the persisted shell theme preference for authenticated views.
export function ThemeProvider({ children }: ThemeProviderProps) {
  const [themeMode, setThemeMode] = useState<ThemeMode>(() => readStoredThemeMode());

  useEffect(() => {
    storeThemeMode(themeMode);
  }, [themeMode]);

  const value = useMemo(() => ({ setThemeMode, themeMode }), [themeMode]);

  return <ThemeContext value={value}>{children}</ThemeContext>;
}

// useThemeContext returns the current theme mode and updater from the authenticated shell provider.
export function useThemeContext(): ThemeContextValue {
  const value = useContext(ThemeContext);
  if (!value) {
    throw new Error('useThemeContext must be used within ThemeProvider');
  }

  return value;
}
