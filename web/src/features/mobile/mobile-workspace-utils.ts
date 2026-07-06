export type MobileTab = 'home' | 'accounts' | 'record' | 'reports' | 'imports' | 'me';

export type MeSection = 'index' | 'profile' | 'security';

export type ThemeMode = 'system' | 'light' | 'dark';

export type CategoryDirection = 'income' | 'expense';

export const mobileRoutes: Record<MobileTab, string> = {
  accounts: '/accounts',
  home: '/home',
  imports: '/imports',
  me: '/me',
  record: '/record',
  reports: '/reports/category',
};

const themeStorageKey = 'accountingTheme';

// formatShortDate receives a date and returns the UTC header date.
export function formatShortDate(value: Date): string {
  return new Intl.DateTimeFormat('en-US', {
    month: 'short',
    day: '2-digit',
    timeZone: 'UTC',
  }).format(value);
}

// categoryDirection receives an entry type and returns the category direction needed for fallback category creation.
export function categoryDirection(type: string): CategoryDirection {
  return type === 'income' || type === 'borrow' ? 'income' : 'expense';
}

// readStoredThemeMode receives no parameters and returns the persisted theme preference.
export function readStoredThemeMode(): ThemeMode {
  try {
    const stored = window.localStorage.getItem(themeStorageKey);
    return stored === 'light' || stored === 'dark' || stored === 'system' ? stored : 'system';
  } catch {
    return 'system';
  }
}

// storeThemeMode receives the selected theme and persists it for later visits.
export function storeThemeMode(mode: ThemeMode) {
  try {
    window.localStorage.setItem(themeStorageKey, mode);
  } catch {
    // Storage can be unavailable in private browsing or tests; theme selection still works in memory.
  }
}
