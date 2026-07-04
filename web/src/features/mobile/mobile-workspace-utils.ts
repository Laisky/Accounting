export type MobileTab = 'home' | 'accounts' | 'record' | 'reports' | 'imports' | 'me';

export type MeSection = 'index' | 'profile' | 'security';

export type ThemeMode = 'system' | 'light' | 'dark';

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
export function categoryDirection(type: string): string {
  return type === 'income' || type === 'borrow' ? 'income' : 'expense';
}

// mobileTabFromPath receives a browser path and returns the authenticated page it addresses.
export function mobileTabFromPath(pathname: string): MobileTab | null {
  if (pathname === '/' || pathname === '/home') {
    return 'home';
  }
  if (pathname === '/accounts' || pathname.startsWith('/accounts/')) {
    return 'accounts';
  }
  if (pathname === '/record' || pathname === '/imports' || pathname === '/me') {
    return pathname.slice(1) as MobileTab;
  }
  if (pathname.startsWith('/me/')) {
    return 'me';
  }
  if (pathname === '/reports' || pathname.startsWith('/reports/')) {
    return 'reports';
  }
  return null;
}

// meSectionFromPath receives a browser path and returns the addressed Me subpage.
export function meSectionFromPath(pathname: string): MeSection | null {
  if (pathname === '/me') {
    return 'index';
  }
  if (pathname === '/me/profile') {
    return 'profile';
  }
  if (pathname === '/me/security') {
    return 'security';
  }

  return null;
}

// accountIdFromTransactionsPath receives a browser path and returns the addressed account id.
export function accountIdFromTransactionsPath(pathname: string): string | null {
  const match = /^\/accounts\/([^/]+)\/transactions$/.exec(pathname);
  if (!match) {
    return null;
  }

  try {
    return decodeURIComponent(match[1]);
  } catch {
    return null;
  }
}

// entryIdFromDetailPath receives a browser path and returns the addressed entry id.
export function entryIdFromDetailPath(pathname: string): string | null {
  const match = /^\/entries\/([^/]+)$/.exec(pathname);
  if (!match) {
    return null;
  }

  try {
    return decodeURIComponent(match[1]);
  } catch {
    return null;
  }
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
