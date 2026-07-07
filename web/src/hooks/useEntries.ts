import { useMemo } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { accountEntries } from '@/features/mobile/account-transaction-utils';
import {
  createEntry,
  deleteEntry,
  fetchAllEntries,
  fetchEntries,
  updateEntry,
  type Entry,
  type EntryList,
  type EntryUpdateInput,
} from '@/lib/api/ledger';
import type { components } from '@/lib/api/generated/schema';
import { queryKeys } from './queryKeys';

type EntryCreateRequest = components['schemas']['EntryCreateRequest'];

// invalidateEntryScopes refreshes every server view that a single entry change can move:
// the book's entries (all/recent/detail), the cross-book balance summary, and reports.
function invalidateEntryScopes(queryClient: ReturnType<typeof useQueryClient>, bookId: string) {
  void queryClient.invalidateQueries({ queryKey: ['books', bookId, 'entries'] });
  void queryClient.invalidateQueries({ queryKey: queryKeys.ledger.summary() });
  void queryClient.invalidateQueries({ queryKey: ['reports'] });
}

// useAllEntriesQuery is the single canonical full-ledger query for a book. Search,
// account-detail, entry-detail, and reports all read from this one cache entry, so a
// book never triggers more than one full-ledger download.
export function useAllEntriesQuery(bookId: string | undefined, enabled = true) {
  return useQuery({
    enabled: Boolean(bookId) && enabled,
    queryFn: () => fetchAllEntries(bookId ?? ''),
    queryKey: queryKeys.entries.all(bookId ?? 'none'),
  });
}

// useRecentEntriesQuery loads the light first page of entries for the home and record views.
export function useRecentEntriesQuery(bookId: string | undefined, pageSize = 20) {
  return useQuery({
    enabled: Boolean(bookId),
    queryFn: (): Promise<EntryList> => fetchEntries(bookId ?? ''),
    queryKey: queryKeys.entries.recent(bookId ?? 'none', pageSize),
  });
}

// useAccountEntries derives an account's entries from the shared full-ledger query.
export function useAccountEntries(bookId: string | undefined, accountId: string | null) {
  const query = useAllEntriesQuery(bookId, Boolean(accountId));
  const entries = useMemo(
    () => (accountId ? accountEntries(accountId, query.data ?? []) : []),
    [accountId, query.data],
  );

  return { entries, isError: query.isError, isLoading: query.isLoading };
}

// useEntryDetail resolves one entry from route-provided data or the shared full-ledger query.
export function useEntryDetail(bookId: string | undefined, entryId: string | null, initialEntry?: Entry) {
  const query = useAllEntriesQuery(bookId, Boolean(entryId) && !initialEntry);
  const resolved = useMemo(
    () => initialEntry ?? query.data?.find((entry) => entry.id === entryId),
    [entryId, initialEntry, query.data],
  );

  return { entry: resolved, isError: query.isError, isLoading: query.isLoading && !resolved };
}

// useCreateEntryMutation records an entry and refreshes entries, summary, and reports.
export function useCreateEntryMutation() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ bookId, input }: { bookId: string; input: EntryCreateRequest }): Promise<Entry> =>
      createEntry(bookId, input),
    onSuccess: (_entry, { bookId }) => invalidateEntryScopes(queryClient, bookId),
  });
}

// useUpdateEntryMutation patches an entry and refreshes entries, summary, and reports.
export function useUpdateEntryMutation() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      bookId,
      entryId,
      input,
    }: {
      bookId: string;
      entryId: string;
      input: EntryUpdateInput;
    }): Promise<Entry> => updateEntry(bookId, entryId, input),
    onSuccess: (_entry, { bookId }) => invalidateEntryScopes(queryClient, bookId),
  });
}

// useDeleteEntryMutation removes an entry and refreshes entries, summary, and reports.
export function useDeleteEntryMutation() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ bookId, entryId }: { bookId: string; entryId: string }): Promise<void> =>
      deleteEntry(bookId, entryId),
    onSuccess: (_result, { bookId }) => invalidateEntryScopes(queryClient, bookId),
  });
}
