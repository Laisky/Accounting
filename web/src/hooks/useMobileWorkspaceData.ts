import { useQuery } from '@tanstack/react-query';
import { loadMobileBookContext, loadMobileFoundation } from '@/features/mobile/mobileWorkspaceLoaders';
import { queryKeys } from './queryKeys';

// useMobileFoundationQuery loads the authenticated shell foundation data through the shared Query cache.
export function useMobileFoundationQuery(revision = 0) {
  return useQuery({
    queryFn: loadMobileFoundation,
    queryKey: queryKeys.workspace.foundation(revision),
  });
}

// useMobileBookContextQuery loads selected-book dimensions and recent entries through the shared Query cache.
export function useMobileBookContextQuery(bookId: string, revision = 0) {
  return useQuery({
    enabled: Boolean(bookId),
    queryFn: () => loadMobileBookContext(bookId),
    queryKey: queryKeys.workspace.bookContext(bookId || 'none', revision),
  });
}
