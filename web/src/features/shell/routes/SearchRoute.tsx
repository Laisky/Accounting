import { useLocation, useNavigate, useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';
import { useBook } from '@/contexts/BookContext';
import { TransactionSearchView } from '@/features/mobile/TransactionSearchView';
import { useAccountsQuery } from '@/hooks/useAccounts';
import { useBookMembersQuery } from '@/hooks/useBookMembers';
import { useCategoriesQuery } from '@/hooks/useCategories';
import { useAllEntriesQuery } from '@/hooks/useEntries';

// SearchRoute reads the URL query and searches the shared full-ledger cache for the book.
function SearchRoute() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams] = useSearchParams();
  const query = searchParams.get('query') ?? '';
  const { selectedBook } = useBook();
  const accounts = useAccountsQuery().data ?? [];
  const categories = useCategoriesQuery(selectedBook?.id).data ?? [];
  const members = useBookMembersQuery(selectedBook?.id).data ?? [];
  const entriesQuery = useAllEntriesQuery(selectedBook?.id);

  // handleClose returns to the page that opened search, defaulting to home.
  function handleClose() {
    navigate(returnToPath(location.state) ?? '/home');
  }

  // handleQueryChange keeps the search text in the URL so results are shareable and refresh-safe.
  function handleQueryChange(next: string) {
    const search = next ? `?${new URLSearchParams({ query: next }).toString()}` : '';
    navigate({ pathname: '/search', search }, { replace: true, state: location.state });
  }

  return (
    <TransactionSearchView
      accounts={accounts}
      categories={categories}
      entries={entriesQuery.data ?? []}
      error={entriesQuery.isError ? t('mobile.search.error') : ''}
      isLoading={entriesQuery.isFetching}
      members={members}
      onClose={handleClose}
      onOpenEntry={(entryId) => navigate(`/entries/${encodeURIComponent(entryId)}`)}
      onQueryChange={handleQueryChange}
      query={query}
    />
  );
}

// returnToPath extracts a safe non-search return path from router navigation state.
function returnToPath(state: unknown): string | null {
  if (!state || typeof state !== 'object' || !('returnTo' in state)) {
    return null;
  }
  const returnTo = (state as { returnTo?: unknown }).returnTo;
  return typeof returnTo === 'string' && returnTo.startsWith('/') && !returnTo.startsWith('/search') ? returnTo : null;
}

export default SearchRoute;
