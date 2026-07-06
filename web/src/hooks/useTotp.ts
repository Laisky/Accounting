import { useQuery } from '@tanstack/react-query';
import { fetchTotpStatus } from '@/lib/api/auth';
import { queryKeys } from './queryKeys';

export const totpStatusQueryKey = queryKeys.auth.totpStatus();

// useTotpStatusQuery loads signed-in TOTP status through the shared Query cache.
export function useTotpStatusQuery(enabled = true) {
  return useQuery({
    enabled,
    queryFn: ({ signal }) => fetchTotpStatus(signal),
    queryKey: totpStatusQueryKey,
  });
}
