import { useQuery } from '@tanstack/react-query';
import { fetchAuditEvents } from '@/lib/api/audit';
import { queryKeys } from './queryKeys';

// useAuditEventsQuery loads recent sanitized audit events through the shared Query cache.
export function useAuditEventsQuery({ enabled = false }: { enabled?: boolean } = {}) {
  return useQuery({
    enabled,
    queryFn: fetchAuditEvents,
    queryKey: queryKeys.audit.list({ page: 1, pageSize: 20 }),
  });
}
