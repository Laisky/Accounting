export type AuditEvent = {
  id: string;
  actorId?: string;
  actorEmail?: string;
  action: string;
  targetType: string;
  targetId?: string;
  metadata?: Record<string, string>;
  createdAt: string;
};

export type AuditEventList = {
  items: AuditEvent[];
  page: number;
  pageSize: number;
  total: number;
};

// fetchAuditEvents receives no parameters, loads recent sanitized audit events, and returns a paginated list.
export async function fetchAuditEvents(): Promise<AuditEventList> {
  const response = await fetch('/api/audit?page=1&page_size=20');
  if (!response.ok) {
    throw new Error(`audit request failed: ${response.status}`);
  }

  return response.json() as Promise<AuditEventList>;
}
