import type { Entry } from './ledger';

export type ImportPreviewRow = {
  rowNumber: number;
  raw?: Record<string, string>;
  type?: string;
  occurredAt?: string;
  account?: string;
  category?: string;
  book?: string;
  member?: string;
  merchant?: string;
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

// previewWacaiImport receives a CSV file and uploads it to the Wacai preview API.
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
export async function applyWacaiImport(bookId: string, batch: ImportPreviewBatch, signal?: AbortSignal): Promise<ImportApplyResponse> {
  const response = await fetch(`/api/books/${encodeURIComponent(bookId)}/imports/${encodeURIComponent(batch.id)}/apply`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ sourceHash: batch.sourceHash }),
    signal,
  });
  if (!response.ok) {
    throw new Error(`import apply failed: ${response.status}`);
  }

  return response.json() as Promise<ImportApplyResponse>;
}
