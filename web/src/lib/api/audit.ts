import { apiRequest } from '@/lib/apiClient';
import type { components } from '@/lib/api/generated/schema';

type Schemas = components['schemas'];

export type AuditEvent = Schemas['AuditEvent'];
export type AuditEventList = Schemas['AuditEventList'];

// fetchAuditEvents receives no parameters, loads recent sanitized audit events, and returns a paginated list.
export async function fetchAuditEvents(): Promise<AuditEventList> {
  return apiRequest<AuditEventList>('/api/audit?page=1&page_size=20');
}
