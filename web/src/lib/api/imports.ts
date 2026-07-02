import type { Entry } from './ledger';

export type ImportPreviewRow = {
  rowNumber: number;
  raw?: Record<string, string>;
  type?: string;
  sourceType?: string;
  occurredAt?: string;
  account?: string;
  destinationAccount?: string;
  category?: string;
  book?: string;
  member?: string;
  participants?: string[];
  merchant?: string;
  attribute?: string;
  note?: string;
  amount?: string;
  currency?: string;
  tags?: string[];
  warnings?: string[];
  errors?: string[];
};

export type ImportPreviewBatch = {
  id: string;
  source: string;
  filename: string;
  sourceHash: string;
  parserVersion?: string;
  detectedSchema?: {
    columns?: Record<string, string>;
    missing?: string[];
  };
  rows: ImportPreviewRow[];
  errorCount: number;
  warningCount: number;
  detected: {
    books?: string[];
    accounts?: string[];
    categories?: string[];
    currencies?: string[];
    members?: string[];
    merchants?: string[];
    tags?: string[];
  };
};

export type ImportApplyResponse = {
  batchId: string;
  bookId: string;
  status: string;
  importedCount: number;
  skippedCount: number;
  entries: Entry[];
  skippedRows?: Array<{ rowNumber: number; reason: string }>;
};

export type ImportApplyOptions = {
  memberMappings?: Record<string, string>;
  signal?: AbortSignal;
};

// previewWacaiImport receives a CSV or XLSX file and uploads it to the Wacai preview API.
export async function previewWacaiImport(file: File, signal?: AbortSignal): Promise<ImportPreviewBatch> {
  const form = new FormData();
  form.append('file', file);

  const response = await fetch('/api/imports/wacai/preview', {
    method: 'POST',
    body: form,
    signal,
  });
  if (!response.ok) {
    throw new Error(`import preview failed: ${response.status}`);
  }

  return response.json() as Promise<ImportPreviewBatch>;
}

// applyWacaiImport receives a book and preview batch fingerprint, then commits mapped rows into ledger entries.
export async function applyWacaiImport(
  bookId: string,
  batch: ImportPreviewBatch,
  options?: AbortSignal | ImportApplyOptions,
): Promise<ImportApplyResponse> {
  const signal = options instanceof AbortSignal ? options : options?.signal;
  const memberMappings = options instanceof AbortSignal ? undefined : options?.memberMappings;
  const body: { sourceHash: string; memberMappings?: Record<string, string> } = { sourceHash: batch.sourceHash };
  if (memberMappings && Object.keys(memberMappings).length > 0) {
    body.memberMappings = memberMappings;
  }

  const response = await fetch(`/api/books/${encodeURIComponent(bookId)}/imports/${encodeURIComponent(batch.id)}/apply`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
    signal,
  });
  if (!response.ok) {
    throw new Error(`import apply failed: ${response.status}`);
  }

  return response.json() as Promise<ImportApplyResponse>;
}
