import {
  Activity,
  AlertCircle,
  CheckCircle2,
  Database,
  FileSpreadsheet,
  HardDriveUpload,
  UploadCloud,
} from 'lucide-react';
import type { TFunction } from 'i18next';
import { type ChangeEvent, type DragEvent, type ReactNode, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { fetchAuditEvents, type AuditEvent } from '../../lib/api/audit';
import { type AuthActor } from '../../lib/api/auth';
import { previewWacaiImport, type ImportPreviewBatch } from '../../lib/api/imports';
import { emptyLedgerSummary, fetchLedgerSummary, type LedgerSummary } from '../../lib/api/ledger';
import { emptyRuntimeConfig, type RuntimeConfig } from '../../lib/api/runtimeConfig';
import { LedgerWorkspace } from '../ledger/LedgerWorkspace';
import { ReportWorkspace } from '../reports/ReportWorkspace';

type ImportSource = {
  id: string;
  acceptedFormats: string;
  enabled: boolean;
};

type ImportStage = 'empty' | 'ready' | 'staging' | 'staged' | 'failed';

type ImportWorkspaceProps = {
  actor: AuthActor;
  runtimeConfig: RuntimeConfig | null;
  onLogout: () => Promise<void>;
};

const importSources: ImportSource[] = [
  {
    id: 'wacai',
    acceptedFormats: '.csv',
    enabled: true,
  },
];

// ImportWorkspace renders the main accounting workspace and returns the first import flow.
export function ImportWorkspace({ actor, runtimeConfig, onLogout }: ImportWorkspaceProps) {
  const { t } = useTranslation();
  const [ledgerRefreshKey, setLedgerRefreshKey] = useState(0);
  const summary = useSummary(ledgerRefreshKey);
  const [selectedSourceID, setSelectedSourceID] = useState(importSources[0].id);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [stage, setStage] = useState<ImportStage>('empty');
  const [previewBatch, setPreviewBatch] = useState<ImportPreviewBatch | null>(null);
  const [importError, setImportError] = useState('');
  const [isLoggingOut, setIsLoggingOut] = useState(false);
  const [isActivityOpen, setIsActivityOpen] = useState(false);
  const [isActivityLoading, setIsActivityLoading] = useState(false);
  const [activityEvents, setActivityEvents] = useState<AuditEvent[]>([]);
  const [activityTotal, setActivityTotal] = useState(0);
  const [activityError, setActivityError] = useState('');
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const selectedSource = importSources.find((source) => source.id === selectedSourceID) ?? importSources[0];
  const selectedSourceLabel = t(`imports.sources.${selectedSource.id}.label`);
  const balance = formatMoney(summary?.balanceCents ?? 0, summary?.currency ?? 'USD');
  const fileSummary = useMemo(() => describeImportFile(t, selectedFile), [selectedFile, t]);
  const rowEstimate = selectedFile ? estimateRows(selectedFile.size) : 0;

  // handleFileChange receives a file input event, stores the first selected file, and returns no value.
  function handleFileChange(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0] ?? null;
    setSelectedFile(file);
    setPreviewBatch(null);
    setImportError('');
    setStage(file ? 'ready' : 'empty');
  }

  // handleDrop receives a drag event, stores the first dropped spreadsheet file, and returns no value.
  function handleDrop(event: DragEvent<HTMLLabelElement>) {
    event.preventDefault();
    const file = event.dataTransfer.files[0] ?? null;
    if (fileInputRef.current) {
      fileInputRef.current.value = '';
    }
    setSelectedFile(file);
    setPreviewBatch(null);
    setImportError('');
    setStage(file ? 'ready' : 'empty');
  }

  // clearImportSelection receives no parameters, clears selected import state, and returns no value.
  function clearImportSelection() {
    setSelectedFile(null);
    setPreviewBatch(null);
    setImportError('');
    setStage('empty');
    if (fileInputRef.current) {
      fileInputRef.current.value = '';
    }
  }

  // handleStageImport accepts no parameters, uploads the selected file for preview, and returns no value.
  async function handleStageImport() {
    if (!selectedFile) {
      return;
    }

    setStage('staging');
    setImportError('');
    try {
      const batch = await previewWacaiImport(selectedFile);
      setPreviewBatch(batch);
      setStage('staged');
    } catch {
      setPreviewBatch(null);
      setImportError(t('imports.error.previewFailed'));
      setStage('failed');
    }
  }

  // handleLogoutClick receives no parameters, clears the browser session, and returns no value.
  async function handleLogoutClick() {
    setIsLoggingOut(true);
    try {
      await onLogout();
    } finally {
      setIsLoggingOut(false);
    }
  }

  // handleActivityClick receives no parameters, toggles and refreshes the recent activity panel.
  async function handleActivityClick() {
    if (isActivityOpen) {
      setIsActivityOpen(false);
      return;
    }

    setIsActivityOpen(true);
    setIsActivityLoading(true);
    setActivityError('');
    try {
      const result = await fetchAuditEvents();
      setActivityEvents(result.items);
      setActivityTotal(result.total);
    } catch {
      setActivityEvents([]);
      setActivityTotal(0);
      setActivityError(t('common.error.activityFailed'));
    } finally {
      setIsActivityLoading(false);
    }
  }

  return (
    <main className="shell">
      <section className="workspace">
        <header className="masthead">
          <div>
            <p className="eyebrow">{t('imports.eyebrow')}</p>
            <h1>{t('imports.heading')}</h1>
            <AuthMethods runtimeConfig={runtimeConfig} />
          </div>
          <div className="sessionTools" aria-label={t('imports.a11y.currentSession')}>
            <span>{actor.email}</span>
            <button
              className="iconButton"
              type="button"
              aria-label={isActivityOpen ? t('imports.a11y.closeActivity') : t('imports.a11y.openActivity')}
              aria-expanded={isActivityOpen}
              onClick={handleActivityClick}
            >
              <Activity size={20} />
            </button>
            <button className="ghostButton" type="button" disabled={isLoggingOut} onClick={handleLogoutClick}>
              {t('common.signOut')}
            </button>
          </div>
        </header>

        {isActivityOpen ? (
          <section className="activityPanel" aria-label={t('imports.a11y.activityLog')}>
            <div>
              <p className="eyebrow">{t('imports.recentActivity')}</p>
              <h2>{t('imports.auditEvents')}</h2>
            </div>
            {isActivityLoading ? <p className="authStatus">{t('common.loadingActivity')}</p> : null}
            {activityError ? <p className="authError">{activityError}</p> : null}
            {!isActivityLoading && !activityError ? (
              activityEvents.length ? (
                <ul className="activityList">
                  {activityEvents.map((event) => (
                    <li key={event.id}>
                      <div>
                        <strong>{formatAuditAction(event.action)}</strong>
                        <span>{event.targetType}</span>
                      </div>
                      <time dateTime={event.createdAt}>{formatAuditTime(event.createdAt)}</time>
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="activityEmpty">{t('imports.noAuditEvents')}</p>
              )
            ) : null}
            <p className="activityTotal">{t('imports.totalEvents', { value: activityTotal })}</p>
          </section>
        ) : null}

        <section className="summaryBand" aria-label={t('imports.a11y.ledgerSummary')}>
          <div className="summaryItem summaryBalance">
            <span>{t('common.balance')}</span>
            <strong>{balance}</strong>
          </div>
          <div className="summaryItem">
            <span>{t('common.currency')}</span>
            <strong>{summary?.currency ?? 'USD'}</strong>
          </div>
          <div className="summaryItem">
            <span>{t('common.entries')}</span>
            <strong>{summary?.entryCount ?? 0}</strong>
          </div>
        </section>

        <ReportWorkspace refreshKey={ledgerRefreshKey} />

        <LedgerWorkspace onLedgerChanged={() => setLedgerRefreshKey((current) => current + 1)} />

        <section className="importLayout" aria-label={t('imports.a11y.importData')}>
          <div className="importMain">
            <div className="sectionHeading">
              <div>
                <p className="eyebrow">{t('imports.dataImport')}</p>
                <h2>{t('imports.chooseSource')}</h2>
              </div>
              <span className="sourceBadge">{t('imports.csvPreviewReady')}</span>
            </div>

            <div className="sourceField">
              <label htmlFor="source">{t('imports.dataSource')}</label>
              <select
                id="source"
                value={selectedSourceID}
                onChange={(event) => setSelectedSourceID(event.target.value)}
              >
                {importSources.map((source) => (
                  <option key={source.id} disabled={!source.enabled} value={source.id}>
                    {t(`imports.sources.${source.id}.label`)}
                  </option>
                ))}
              </select>
              <p>{t(`imports.sources.${selectedSource.id}.description`)}</p>
            </div>

            <label
              className={`dropZone ${selectedFile ? 'dropZoneReady' : ''}`}
              onDragOver={(event) => event.preventDefault()}
              onDrop={handleDrop}
            >
              <input
                ref={fileInputRef}
                type="file"
                accept={selectedSource.acceptedFormats}
                aria-label={t('imports.a11y.uploadFile')}
                onChange={handleFileChange}
              />
              <UploadCloud size={34} />
              <span>{selectedFile ? selectedFile.name : t('imports.uploadPrompt')}</span>
              <small>{selectedFile ? fileSummary : t('imports.acceptedFormat')}</small>
            </label>

            <div className="actionRow">
              <button
                className="primaryButton"
                type="button"
                disabled={!selectedFile || stage === 'staging'}
                onClick={handleStageImport}
              >
                <HardDriveUpload size={18} />
                <span>{stage === 'staging' ? t('imports.stage.staging') : stage === 'staged' ? t('imports.stage.staged') : t('imports.stage.stage')}</span>
              </button>
              <button
                className="ghostButton"
                type="button"
                disabled={!selectedFile}
                onClick={clearImportSelection}
              >
                {t('imports.clearFile')}
              </button>
            </div>
          </div>

          <aside className="reviewPanel" aria-label={t('imports.a11y.importReview')}>
            <div className="reviewHeader">
              <FileSpreadsheet size={22} />
              <div>
                <h3>{t('imports.reviewQueue')}</h3>
                <p>{stage === 'empty' ? t('imports.noFileSelected') : t('imports.exportDetected', { source: selectedSourceLabel })}</p>
              </div>
            </div>

            <dl className="fileFacts">
              <div>
                <dt>{t('imports.facts.source')}</dt>
                <dd>{selectedSourceLabel}</dd>
              </div>
              <div>
                <dt>{t('imports.facts.rows')}</dt>
                <dd>{previewBatch ? previewBatch.rows.length : rowEstimate ? `~${rowEstimate}` : t('imports.facts.pending')}</dd>
              </div>
              <div>
                <dt>{t('imports.facts.fingerprint')}</dt>
                <dd>{previewBatch ? previewBatch.sourceHash.slice(0, 8) : t('imports.facts.pendingHash')}</dd>
              </div>
              <div>
                <dt>{t('imports.facts.metadata')}</dt>
                <dd>{previewBatch ? t('imports.facts.batch', { id: previewBatch.id.slice(0, 8) }) : t('imports.facts.previewBatch')}</dd>
              </div>
            </dl>

            {importError ? <p className="importError">{importError}</p> : null}
            {previewBatch ? (
              <>
                <div className="previewSummary" aria-label={t('imports.a11y.previewSummary')}>
                  <span>{t('imports.preview.warnings', { value: previewBatch.warningCount })}</span>
                  <span>{t('imports.preview.errors', { value: previewBatch.errorCount })}</span>
                  <span>{t('imports.preview.accounts', { value: previewBatch.detected.accounts?.length ?? 0 })}</span>
                  <span>{t('imports.preview.categories', { value: previewBatch.detected.categories?.length ?? 0 })}</span>
                </div>
                <PreviewDetectedValues batch={previewBatch} />
                <PreviewRows batch={previewBatch} />
              </>
            ) : null}

            <ol className="pipeline">
              <PipelineStep
                active={stage !== 'empty'}
                icon={<CheckCircle2 size={18} />}
                label={t('imports.pipeline.validate')}
              />
              <PipelineStep
                active={stage === 'staging' || stage === 'staged'}
                icon={<HardDriveUpload size={18} />}
                label={t('imports.pipeline.hash')}
              />
              <PipelineStep
                active={stage === 'staged'}
                icon={<Database size={18} />}
                label={t('imports.pipeline.batch')}
              />
              <PipelineStep
                active={false}
                icon={<AlertCircle size={18} />}
                label={t('imports.pipeline.map')}
              />
            </ol>
          </aside>
        </section>

      </section>
    </main>
  );
}

// AuthMethods receives runtime config and returns compact public sign-in capability labels.
function AuthMethods({ runtimeConfig }: { runtimeConfig: RuntimeConfig | null }) {
  const { t } = useTranslation();
  const config = runtimeConfig ?? emptyRuntimeConfig;
  const methods = [
    config.auth.emailLoginEnabled ? t('auth.methods.emailLogin') : t('auth.methods.emailLoginOff'),
    config.auth.emailRegisterEnabled ? t('auth.methods.registrationOpen') : t('auth.methods.registrationClosed'),
    config.features.externalSsoEnabled ? t('auth.methods.externalSso') : '',
    config.features.passkeyEnabled ? t('auth.methods.passkeys') : '',
    config.features.totpEnabled ? t('auth.methods.totp') : '',
    config.features.turnstileEnabled ? t('auth.methods.turnstile') : '',
  ].filter(Boolean);

  return (
    <div className="authMethods" aria-label={t('auth.a11y.availableLoginMethods')}>
      {methods.map((method) => (
        <span key={method}>{method}</span>
      ))}
    </div>
  );
}

// PipelineStep receives milestone display properties and returns one backend processing step in the import queue.
function PipelineStep({ active, icon, label }: { active: boolean; icon: ReactNode; label: string }) {
  return (
    <li className={active ? 'pipelineActive' : ''}>
      <span>{icon}</span>
      <p>{label}</p>
    </li>
  );
}

// formatAuditAction receives an audit action string and returns readable activity text.
function formatAuditAction(action: string): string {
  return action
    .split('.')
    .map((part) => part.replace(/_/g, ' '))
    .join(' / ');
}

// formatAuditTime receives an ISO timestamp and returns a compact UTC display value.
function formatAuditTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return date.toISOString().replace('.000Z', 'Z');
}

// PreviewDetectedValues receives a preview batch and returns detected import migration concepts.
function PreviewDetectedValues({ batch }: { batch: ImportPreviewBatch }) {
  const { t } = useTranslation();
  const detectedGroups = [
    { label: t('imports.detected.books'), values: batch.detected.books },
    { label: t('imports.detected.accounts'), values: batch.detected.accounts },
    { label: t('imports.detected.categories'), values: batch.detected.categories },
    { label: t('imports.detected.currencies'), values: batch.detected.currencies },
    { label: t('imports.detected.members'), values: batch.detected.members },
    { label: t('imports.detected.merchants'), values: batch.detected.merchants },
    { label: t('imports.detected.tags'), values: batch.detected.tags },
  ];
  const missing = batch.detectedSchema?.missing ?? [];

  return (
    <section className="previewDetails" aria-label={t('imports.a11y.detectedValues')}>
      {missing.length ? (
        <p className="schemaNotice">{t('imports.missingFields', { fields: missing.join(', ') })}</p>
      ) : (
        <p className="schemaNotice">{t('imports.schemaReady')}</p>
      )}
      <div className="detectedGrid">
        {detectedGroups.map((group) => (
          <div key={group.label}>
            <span>{group.label}</span>
            <strong>{compactValues(t, group.values)}</strong>
          </div>
        ))}
      </div>
    </section>
  );
}

// PreviewRows receives a preview batch and returns a compact row-level diagnostics review.
function PreviewRows({ batch }: { batch: ImportPreviewBatch }) {
  const { t } = useTranslation();
  const rows = batch.rows.slice(0, 4);

  return (
    <section className="rowDiagnostics" aria-label={t('imports.a11y.rowDiagnostics')}>
      <h4>{t('imports.rowDiagnostics')}</h4>
      {rows.length ? (
        <ul>
          {rows.map((row) => {
            const diagnostics = [...(row.errors ?? []), ...(row.warnings ?? [])];
            return (
              <li key={row.rowNumber}>
                <span>{t('imports.rowNumber', { value: row.rowNumber })}</span>
                <strong>{row.book || row.account || row.category || row.type || t('imports.unmappedRow')}</strong>
                <small>{diagnostics.length ? diagnostics.join('; ') : t('imports.readyForMapping')}</small>
              </li>
            );
          })}
        </ul>
      ) : (
        <p>{t('imports.noPreviewRows')}</p>
      )}
    </section>
  );
}

// compactValues receives the translator and optional detected values and returns a concise readable summary.
function compactValues(t: TFunction, values?: string[]): string {
  if (!values?.length) {
    return t('imports.none');
  }

  if (values.length <= 2) {
    return values.join(', ');
  }

  return `${values.slice(0, 2).join(', ')} +${values.length - 2}`;
}

// formatMoney receives cents and an ISO currency code and returns localized currency text.
function formatMoney(cents: number, currencyCode: string): string {
  return moneyFormatter(currencyCode).format(cents / 100);
}

// moneyFormatter receives an ISO currency code and returns a safe localized formatter.
function moneyFormatter(currencyCode: string): Intl.NumberFormat {
  try {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: currencyCode,
    });
  } catch {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
    });
  }
}

// useSummary accepts a refresh key, loads the current ledger summary, and returns a fallback on request failure.
function useSummary(refreshKey: number): LedgerSummary | null {
  const [summary, setSummary] = useState<LedgerSummary | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    fetchLedgerSummary(controller.signal)
      .then(setSummary)
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === 'AbortError') {
          return;
        }
        setSummary(emptyLedgerSummary);
      });

    return () => controller.abort();
  }, [refreshKey]);

  return summary;
}

// describeImportFile receives the translator and a selected file and returns formatted upload summary text.
function describeImportFile(t: TFunction, file: File | null): string {
  if (!file) {
    return t('imports.noFileSelected');
  }

  const sizeInMB = file.size / 1024 / 1024;
  return t('imports.fileSummary', { type: file.type || t('imports.spreadsheet'), size: sizeInMB.toFixed(2) });
}

// estimateRows receives a file size in bytes and returns a conservative row estimate before parsing.
function estimateRows(size: number): number {
  if (size <= 0) {
    return 1;
  }

  return Math.max(12, Math.round(size / 180));
}
