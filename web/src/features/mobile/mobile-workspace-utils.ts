export type MobileTab = 'home' | 'accounts' | 'record' | 'reports' | 'imports' | 'me';

export const mobileRoutes: Record<MobileTab, string> = {
  accounts: '/accounts',
  home: '/home',
  imports: '/imports',
  me: '/me',
  record: '/record',
  reports: '/reports/category',
};

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
  if (pathname === '/reports' || pathname.startsWith('/reports/')) {
    return 'reports';
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
