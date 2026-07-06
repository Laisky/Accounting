import { useQuery } from '@tanstack/react-query';
import { fetchSession } from '@/lib/api/auth';
import { queryKeys } from './queryKeys';

// useSessionQuery loads the current browser session through the shared Query cache.
export function useSessionQuery() {
  return useQuery({
    queryFn: ({ signal }) => fetchSession(signal),
    queryKey: queryKeys.auth.session(),
  });
}
