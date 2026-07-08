import { apiRequest } from '@/lib/apiClient';
import type { components } from '@/lib/api/generated/schema';

type Schemas = components['schemas'];

export type ImportPreviewBatch = Schemas['ImportPreviewBatch'];
export type ImportApplyResponse = Schemas['ImportApplyResponse'];

export type ImportApplyOptions = {
  batch: ImportPreviewBatch;
  bookId: string;
  memberMappings?: Record<string, string>;
  signal?: AbortSignal;
};

// previewWacaiImport receives a CSV or XLSX file and uploads it to the Wacai preview API.
export async function previewWacaiImport(file: File, signal?: AbortSignal): Promise<ImportPreviewBatch> {
  const form = new FormData();
  form.append('file', file);

  return apiRequest<ImportPreviewBatch>('/api/v1/imports/wacai/preview', { method: 'POST', body: form, signal });
}

// applyWacaiImport receives a book and preview batch fingerprint, then commits mapped rows into ledger entries.
export async function applyWacaiImport(options: ImportApplyOptions): Promise<ImportApplyResponse> {
  const body: Schemas['ImportApplyRequest'] = { sourceHash: options.batch.sourceHash };
  if (options.memberMappings && Object.keys(options.memberMappings).length > 0) {
    body.memberMappings = options.memberMappings;
  }

  return apiRequest<ImportApplyResponse>(
    `/api/v1/books/${encodeURIComponent(options.bookId)}/imports/${encodeURIComponent(options.batch.id)}/apply`,
    {
      method: 'POST',
      body,
      signal: options.signal,
    },
  );
}
