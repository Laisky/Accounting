import { useQuery } from '@tanstack/react-query';
import { fetchAllEntries, type BookListItem, type Entry } from '@/lib/api/ledger';
import { queryKeys } from '@/hooks/queryKeys';

type UseMobileSearchEntriesInput = {
  errorMessage: string;
  isSearchOpen: boolean;
  refreshKey: number;
  selectedBook?: BookListItem;
};

// useMobileSearchEntries loads book entries while the routed transaction search view is active.
export function useMobileSearchEntries({
  errorMessage,
  isSearchOpen,
  refreshKey,
  selectedBook,
}: UseMobileSearchEntriesInput) {
  const query = useQuery<Entry[]>({
    enabled: isSearchOpen && Boolean(selectedBook),
    queryFn: () => (selectedBook ? fetchAllEntries(selectedBook.id) : Promise.resolve([])),
    queryKey: selectedBook
      ? queryKeys.entries.list(selectedBook.id, { pageSize: 100, query: `search-all:${refreshKey}` })
      : queryKeys.entries.list('none', { pageSize: 100, query: `search-all:${refreshKey}` }),
  });

  return {
    isSearchLoading: query.isFetching,
    searchEntries: isSearchOpen ? (query.data ?? []) : [],
    searchError: query.isError ? errorMessage : '',
  };
}
