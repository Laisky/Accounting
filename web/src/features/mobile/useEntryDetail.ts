import { useQuery } from '@tanstack/react-query';
import { queryKeys } from '@/hooks/queryKeys';
import { fetchAllEntries, type Entry } from '@/lib/api/ledger';

type UseEntryDetailOptions = {
  bookId?: string;
  entryId: string | null;
  initialEntry?: Entry;
  refreshKey: number;
};

// useEntryDetail resolves one entry from existing route data or a direct-open Query fallback.
export function useEntryDetail({ bookId, entryId, initialEntry, refreshKey }: UseEntryDetailOptions) {
  const query = useQuery({
    enabled: Boolean(entryId && bookId && !initialEntry),
    queryFn: async () => (await fetchAllEntries(bookId ?? '')).find((entry) => entry.id === entryId),
    queryKey: queryKeys.entries.detail(bookId ?? 'none', entryId ?? 'none', refreshKey),
  });

  return {
    entry: initialEntry ?? query.data,
    isError: query.isError,
    isLoading: query.isLoading,
  };
}
