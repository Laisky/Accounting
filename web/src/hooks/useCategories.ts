import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  createCategory,
  fetchCategories,
  updateCategory,
  type Category,
  type CategoryCreateInput,
  type CategoryUpdateInput,
} from '@/lib/api/ledger';
import { queryKeys } from './queryKeys';

// useCategoriesQuery loads a book's categories through the shared Query cache.
export function useCategoriesQuery(bookId: string | undefined) {
  return useQuery({
    enabled: Boolean(bookId),
    queryFn: () => fetchCategories(bookId ?? ''),
    queryKey: queryKeys.categories.list(bookId ?? 'none'),
  });
}

// useCreateCategoryMutation creates a category and refreshes the book's category list.
export function useCreateCategoryMutation() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ bookId, input }: { bookId: string; input: CategoryCreateInput }): Promise<Category> =>
      createCategory(bookId, input),
    onSuccess: (_category, { bookId }) => {
      void queryClient.invalidateQueries({ queryKey: ['books', bookId, 'categories'] });
    },
  });
}

// useUpdateCategoryMutation patches a category and refreshes the book's category list.
export function useUpdateCategoryMutation() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      bookId,
      categoryId,
      input,
    }: {
      bookId: string;
      categoryId: string;
      input: CategoryUpdateInput;
    }): Promise<Category> => updateCategory(bookId, categoryId, input),
    onSuccess: (_category, { bookId }) => {
      void queryClient.invalidateQueries({ queryKey: ['books', bookId, 'categories'] });
    },
  });
}
