import { useQuery } from '@tanstack/react-query';
import { fetchRuntimeConfig } from '@/lib/api/runtimeConfig';
import { queryKeys } from './queryKeys';

// useRuntimeConfigQuery loads public runtime configuration through the shared Query cache.
export function useRuntimeConfigQuery() {
  return useQuery({
    queryFn: ({ signal }) => fetchRuntimeConfig(signal),
    queryKey: queryKeys.runtimeConfig.public(),
  });
}
