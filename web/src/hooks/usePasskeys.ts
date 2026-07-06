import { useQuery } from '@tanstack/react-query';
import { fetchPasskeys } from '@/lib/api/auth';
import { queryKeys } from './queryKeys';

export const passkeyListQueryKey = queryKeys.auth.passkeys({ page: 1, pageSize: 20 });

// usePasskeysQuery loads signed-in passkey metadata through the shared Query cache.
export function usePasskeysQuery(enabled = true) {
  return useQuery({
    enabled,
    queryFn: ({ signal }) => fetchPasskeys(signal),
    queryKey: passkeyListQueryKey,
  });
}
