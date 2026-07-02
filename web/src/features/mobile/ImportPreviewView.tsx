import { AlertCircle, CheckCircle2, FileSpreadsheet, UploadCloud } from 'lucide-react';
import { type ChangeEvent, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { applyWacaiImport, previewWacaiImport, type ImportApplyResponse, type ImportPreviewBatch } from '../../lib/api/imports';
import './import-preview.css';

type ImportStage = 'empty' | 'ready' | 'staging' | 'staged' | 'applying' | 'applied' | 'failed';

type PreviewMetric = {
  label: string;
  value: string | number;
};

type ImportPreviewViewProps = {
  selectedBookId: string;
  onApplied: () => void;
};

// ImportPreviewView renders the mobile Wacai CSV preview workflow.
export function ImportPreviewView({ selectedBookId, onApplied }: ImportPreviewViewProps) {
  const { t } = useTranslation();
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [stage, setStage] = useState<ImportStage>('empty');
  const [previewBatch, setPreviewBatch] = useState<ImportPreviewBatch | null>(null);
  const [applyResult, setApplyResult] = useState<ImportApplyResponse | null>(null);
  const [error, setError] = useState('');
  const inputRef = useRef<HTMLInputElement | null>(null);
  const metrics = usePreviewMetrics(previewBatch);

  // handleFileChange receives a file input event, stores the selected CSV file, and clears stale preview data.
  function handleFileChange(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0] ?? null;
    setSelectedFile(file);
    setPreviewBatch(null);
    setApplyResult(null);
    setError('');
    setStage(file ? 'ready' : 'empty');
  }

  // handleClearSelection receives no parameters, clears the selected file and preview, and resets the input element.
  function handleClearSelection() {
    setSelectedFile(null);
    setPreviewBatch(null);
    setApplyResult(null);
    setError('');
    setStage('empty');
    if (inputRef.current) {
      inputRef.current.value = '';
    }
  }

  // handleStageImport receives no parameters, uploads the selected file, and stores the returned preview batch.
  async function handleStageImport() {
    if (!selectedFile) {
      return;
    }

    setStage('staging');
    setError('');
    try {
      const batch = await previewWacaiImport(selectedFile);
      setPreviewBatch(batch);
      setApplyResult(null);
      setStage('staged');
    } catch {
      setPreviewBatch(null);
      setError(t('imports.error.previewFailed'));
      setStage('failed');
    }
  }

  // handleApplyImport receives no parameters, applies the staged batch, and refreshes surrounding ledger data.
  async function handleApplyImport() {
    if (!previewBatch || !selectedBookId || previewBatch.errorCount > 0) {
      return;
    }

    setStage('applying');
    setError('');
    try {
      const result = await applyWacaiImport(selectedBookId, previewBatch);
      setApplyResult(result);
      setStage('applied');
      onApplied();
    } catch {
      setError(t('imports.error.applyFailed'));
      setStage('staged');
    }
  }

  return (
    <section className="tabPanel importPanel" aria-label={t('imports.a11y.importData')}>
      <div className="panelIntro">
        <p>{t('imports.dataImport')}</p>
        <h1>{t('imports.chooseSource')}</h1>
      </div>

      <label className={`mobileImportDrop ${selectedFile ? 'mobileImportDropReady' : ''}`}>
        <input
          ref={inputRef}
          type="file"
          accept=".csv"
          aria-label={t('imports.a11y.uploadFile')}
          onChange={handleFileChange}
        />
        <UploadCloud size={28} />
        <strong>{selectedFile?.name ?? t('imports.uploadPrompt')}</strong>
        <span>{selectedFile ? describeFile(selectedFile) : t('imports.acceptedFormat')}</span>
      </label>

      <div className="mobileImportActions">
        <button
          className="mobilePrimaryButton"
          type="button"
          disabled={!selectedFile || stage === 'staging'}
          onClick={handleStageImport}
        >
          {stage === 'staging' ? t('imports.stage.staging') : t('imports.stage.stage')}
        </button>
        <button className="mobileSecondaryButton" type="button" disabled={!selectedFile} onClick={handleClearSelection}>
          {t('imports.clearFile')}
        </button>
      </div>

      {error ? (
        <p className="mobileImportMessage mobileImportMessageError">
          <AlertCircle size={18} />
          {error}
        </p>
      ) : null}

      {previewBatch ? (
        <div className="mobileImportPreview" aria-label={t('imports.a11y.previewSummary')}>
          <p className="mobileImportMessage">
            <CheckCircle2 size={18} />
            {t('imports.stage.staged')}
          </p>

          <div className="mobileImportMetrics">
            {metrics.map((metric) => (
              <div key={metric.label}>
                <span>{metric.label}</span>
                <strong>{metric.value}</strong>
              </div>
            ))}
          </div>

          <DetectedValues batch={previewBatch} />
          <PreviewRows batch={previewBatch} />
          <div className="mobileImportApply">
            <button
              className="mobilePrimaryButton"
              type="button"
              disabled={!selectedBookId || previewBatch.errorCount > 0 || stage === 'applying' || stage === 'applied'}
              onClick={handleApplyImport}
            >
              {stage === 'applying' ? t('imports.stage.applying') : t('imports.stage.apply')}
            </button>
            {applyResult ? (
              <p className="mobileImportMessage">
                <CheckCircle2 size={18} />
                {t('imports.stage.applied', {
                  imported: applyResult.importedCount,
                  skipped: applyResult.skippedCount,
                })}
              </p>
            ) : null}
          </div>
        </div>
      ) : (
        <div className="mobileImportEmpty">
          <FileSpreadsheet size={24} />
          <span>{t('imports.noFileSelected')}</span>
        </div>
      )}
    </section>
  );
}

// DetectedValues receives a preview batch and returns discovered import dimensions.
function DetectedValues({ batch }: { batch: ImportPreviewBatch }) {
  const { t } = useTranslation();
  const values = [
    { key: 'books', label: t('imports.detected.books'), values: batch.detected.books },
    { key: 'accounts', label: t('imports.detected.accounts'), values: batch.detected.accounts },
    { key: 'categories', label: t('imports.detected.categories'), values: batch.detected.categories },
    { key: 'currencies', label: t('imports.detected.currencies'), values: batch.detected.currencies },
  ];

  return (
    <div className="mobileImportDetected" aria-label={t('imports.a11y.detectedValues')}>
      {values.map((item) => (
        <div key={item.key}>
          <span>{item.label}</span>
          <strong>{item.values?.length ? item.values.join(', ') : t('imports.none')}</strong>
        </div>
      ))}
    </div>
  );
}

// PreviewRows receives a preview batch and returns the first parsed rows with diagnostics.
function PreviewRows({ batch }: { batch: ImportPreviewBatch }) {
  const { t } = useTranslation();
  const rows = batch.rows.slice(0, 4);

  if (!rows.length) {
    return <p className="emptyState">{t('imports.noPreviewRows')}</p>;
  }

  return (
    <ul className="mobileImportRows" aria-label={t('imports.a11y.rowDiagnostics')}>
      {rows.map((row) => (
        <li key={row.rowNumber}>
          <div>
            <span>{t('imports.rowNumber', { value: row.rowNumber })}</span>
            <strong>{row.note || row.merchant || row.category || t('imports.unmappedRow')}</strong>
          </div>
          <b>{row.amount ? `${row.currency ?? ''} ${row.amount}`.trim() : t('imports.none')}</b>
          {row.errors?.length || row.warnings?.length ? (
            <small>{[...(row.errors ?? []), ...(row.warnings ?? [])].join('; ')}</small>
          ) : null}
        </li>
      ))}
    </ul>
  );
}

// usePreviewMetrics receives an import preview batch and returns compact metric labels for the mobile summary.
function usePreviewMetrics(batch: ImportPreviewBatch | null): PreviewMetric[] {
  const { t } = useTranslation();

  return useMemo(() => {
    if (!batch) {
      return [];
    }

    return [
      { label: t('imports.facts.rows'), value: batch.rows.length },
      { label: t('imports.preview.warnings', { value: batch.warningCount }), value: batch.warningCount },
      { label: t('imports.preview.errors', { value: batch.errorCount }), value: batch.errorCount },
      { label: t('imports.facts.batch', { id: batch.id.slice(0, 8) }), value: batch.sourceHash.slice(0, 8) },
    ];
  }, [batch, t]);
}

// describeFile receives a selected file and returns a compact display name with size.
function describeFile(file: File): string {
  const size = Math.max(file.size / 1024, 1).toFixed(0);
  return `${file.type || 'text/csv'} - ${size} KB`;
}
