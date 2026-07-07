import { useQuery } from '@tanstack/react-query';
import { fetchBookMembers } from '@/lib/api/ledger';
import { queryKeys } from './queryKeys';

// useBookMembersQuery loads a book's explicit members through the shared Query cache.
export function useBookMembersQuery(bookId: string | undefined) {
  return useQuery({
    enabled: Boolean(bookId),
    queryFn: () => fetchBookMembers(bookId ?? ''),
    queryKey: queryKeys.books.members(bookId ?? 'none'),
  });
}
