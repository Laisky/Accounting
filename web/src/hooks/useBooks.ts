import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { createBook, fetchBooks, updateBook, type BookListItem } from '@/lib/api/ledger';
import { queryKeys } from './queryKeys';

// useBooksQuery loads the actor-visible books through the shared Query cache.
export function useBooksQuery() {
  return useQuery({
    queryFn: fetchBooks,
    queryKey: queryKeys.books.list(),
  });
}

// useCreateBookMutation creates a book and refreshes the book list plus balance summary.
export function useCreateBookMutation() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ name, reportingCurrency }: { name: string; reportingCurrency: string }): Promise<BookListItem> =>
      createBook(name, reportingCurrency),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['books', 'list'] });
      void queryClient.invalidateQueries({ queryKey: queryKeys.ledger.summary() });
    },
  });
}

// useUpdateBookMutation patches a book and refreshes downstream book, summary, and report data.
export function useUpdateBookMutation() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      bookId,
      input,
    }: {
      bookId: string;
      input: { name?: string; reportingCurrency?: string };
    }): Promise<BookListItem> => updateBook(bookId, input),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['books', 'list'] });
      void queryClient.invalidateQueries({ queryKey: queryKeys.ledger.summary() });
      void queryClient.invalidateQueries({ queryKey: ['reports'] });
    },
  });
}
