import { useMatch, useSearchParams } from 'react-router';
import { isReportTab, type ReportTab } from '@/features/reports/reportWorkspaceModel';
import type { MeSection, MobileTab } from '@/features/mobile/mobile-workspace-utils';

export type ShellChrome = {
  accountDetailId: string | null;
  activeTab: MobileTab;
  entryDetailId: string | null;
  isSearchOpen: boolean;
  meSection: MeSection;
  reportTab: ReportTab;
  searchQuery: string;
};

// useShellChrome derives the presentational shell mode from the router match set — no
// manual pathname parsing. Each authenticated route resolves to a header/nav descriptor.
export function useShellChrome(): ShellChrome {
  const [searchParams] = useSearchParams();
  const accountDetail = useMatch('/accounts/:accountId/transactions');
  const accountsList = useMatch('/accounts');
  const entryDetail = useMatch('/entries/:entryId');
  const reports = useMatch('/reports/:dimension');
  const record = useMatch('/record');
  const imports = useMatch('/imports');
  const meProfile = useMatch('/me/profile');
  const meSecurity = useMatch('/me/security');
  const me = useMatch('/me');
  const search = useMatch('/search');

  let activeTab: MobileTab = 'home';
  if (accountsList || accountDetail) {
    activeTab = 'accounts';
  } else if (record) {
    activeTab = 'record';
  } else if (reports) {
    activeTab = 'reports';
  } else if (imports) {
    activeTab = 'imports';
  } else if (me || meProfile || meSecurity) {
    activeTab = 'me';
  }

  const dimension = reports?.params.dimension;

  return {
    accountDetailId: accountDetail?.params.accountId ?? null,
    activeTab,
    entryDetailId: entryDetail?.params.entryId ?? null,
    isSearchOpen: Boolean(search),
    meSection: meProfile ? 'profile' : meSecurity ? 'security' : 'index',
    reportTab: isReportTab(dimension) ? dimension : 'category',
    searchQuery: searchParams.get('query') ?? '',
  };
}
