import { useQuery } from '@tanstack/react-query';
import { queryKeys } from '@/hooks/queryKeys';
import { fetchAllEntries } from '@/lib/api/ledger';
import { accountEntries } from './account-transaction-utils';

type UseAccountDetailEntriesOptions = {
  accountId: string | null;
  bookId?: string;
  refreshKey: number;
};

// useAccountDetailEntries loads account-scoped entries through the shared Query cache.
export function useAccountDetailEntries({ accountId, bookId, refreshKey }: UseAccountDetailEntriesOptions) {
  return useQuery({
    enabled: Boolean(accountId && bookId),
    queryFn: async () => accountEntries(accountId ?? '', await fetchAllEntries(bookId ?? '')),
    queryKey: queryKeys.entries.list(bookId ?? 'none', {
      accountId: accountId ?? undefined,
      page: 1,
      pageSize: 100,
      revision: refreshKey,
    }),
  });
}
