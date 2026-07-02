import { AlertCircle, CheckCircle2, FileSpreadsheet, UploadCloud } from 'lucide-react';
import { type ChangeEvent, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { type AuthActor } from '../../lib/api/auth';
import { applyWacaiImport, previewWacaiImport, type ImportApplyResponse, type ImportPreviewBatch } from '../../lib/api/imports';
import { type BookListItem, type BookMember } from '../../lib/api/ledger';
import './import-preview.css';

type ImportStage = 'empty' | 'ready' | 'staging' | 'staged' | 'applying' | 'applied' | 'failed';

type PreviewMetric = {
  label: string;
  value: string | number;
};

type ImportPreviewViewProps = {
  actor: AuthActor;
  books: BookListItem[];
  members: BookMember[];
  onCreateBook: (name: string) => Promise<string>;
  selectedBookId: string;
  setSelectedBookId: (bookID: string) => void;
  onApplied: () => void;
};

type MemberMappingRequirement = {
  sourceName: string;
  suggestedUserID: string;
};

// ImportPreviewView renders the mobile Wacai preview workflow.
export function ImportPreviewView({
  actor,
  books,
  members,
  onApplied,
  onCreateBook,
  selectedBookId,
  setSelectedBookId,
}: ImportPreviewViewProps) {
  const { t } = useTranslation();
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [stage, setStage] = useState<ImportStage>('empty');
  const [previewBatch, setPreviewBatch] = useState<ImportPreviewBatch | null>(null);
  const [applyResult, setApplyResult] = useState<ImportApplyResponse | null>(null);
  const [memberMappings, setMemberMappings] = useState<Record<string, string>>({});
  const [newBookName, setNewBookName] = useState('');
  const [error, setError] = useState('');
  const [isCreatingBook, setIsCreatingBook] = useState(false);
  const inputRef = useRef<HTMLInputElement | null>(null);
  const metrics = usePreviewMetrics(previewBatch);
  const memberRequirements = useMemberMappingRequirements(previewBatch, actor, members);
  const effectiveMemberMappings = useEffectiveMemberMappings(memberRequirements, memberMappings);
  const hasMissingMemberMappings = memberRequirements.some((requirement) => !effectiveMemberMappings[requirement.sourceName]);
  const suggestedBookName = previewBatch?.detected.books?.[0] ?? '';

  // handleFileChange receives a file input event, stores the selected import file, and clears stale preview data.
  function handleFileChange(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0] ?? null;
    setSelectedFile(file);
    setPreviewBatch(null);
    setApplyResult(null);
    setMemberMappings({});
    setError('');
    setStage(file ? 'ready' : 'empty');
  }

  // handleClearSelection receives no parameters, clears the selected file and preview, and resets the input element.
  function handleClearSelection() {
    setSelectedFile(null);
    setPreviewBatch(null);
    setApplyResult(null);
    setMemberMappings({});
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
      setMemberMappings(defaultMemberMappings(batch, actor, members));
      setStage('staged');
    } catch {
      setPreviewBatch(null);
      setError(t('imports.error.previewFailed'));
      setStage('failed');
    }
  }

  // handleApplyImport receives no parameters, applies the staged batch, and refreshes surrounding ledger data.
  async function handleApplyImport() {
    if (!previewBatch || !selectedBookId || previewBatch.errorCount > 0 || hasMissingMemberMappings) {
      return;
    }

    setStage('applying');
    setError('');
    try {
      const result = await applyWacaiImport(selectedBookId, previewBatch, { memberMappings: effectiveMemberMappings });
      setApplyResult(result);
      setStage('applied');
      onApplied();
    } catch {
      setError(t('imports.error.applyFailed'));
      setStage('staged');
    }
  }

  // handleMemberMappingChange receives a source name and stores the typed uid or email reference.
  function handleMemberMappingChange(sourceName: string, value: string) {
    setMemberMappings((current) => ({ ...current, [sourceName]: value }));
  }

  // handleCreateDestinationBook receives no parameters and creates a destination book from the typed name.
  async function handleCreateDestinationBook() {
    const name = newBookName.trim() || suggestedBookName.trim();
    if (!name) {
      return;
    }

    setIsCreatingBook(true);
    setError('');
    try {
      const bookID = await onCreateBook(name);
      setSelectedBookId(bookID);
      setNewBookName('');
    } catch {
      setError(t('imports.error.bookCreateFailed'));
    } finally {
      setIsCreatingBook(false);
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
          accept=".csv,.xlsx"
          aria-label={t('imports.a11y.uploadFile')}
          onChange={handleFileChange}
        />
        <UploadCloud size={28} />
        <strong>{selectedFile?.name ?? t('imports.uploadPrompt')}</strong>
        <span>{selectedFile ? describeFile(selectedFile, t('imports.spreadsheet')) : t('imports.acceptedFormat')}</span>
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

      <BookDestination
        books={books}
        isCreatingBook={isCreatingBook}
        newBookName={newBookName}
        selectedBookId={selectedBookId}
        suggestedBookName={suggestedBookName}
        onCreateBook={handleCreateDestinationBook}
        onNewBookNameChange={setNewBookName}
        onSelectBook={setSelectedBookId}
      />

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
          <MemberMappings
            mappings={memberMappings}
            requirements={memberRequirements}
            onChange={handleMemberMappingChange}
          />
          <PreviewRows batch={previewBatch} />
          <div className="mobileImportApply">
            <button
              className="mobilePrimaryButton"
              type="button"
              disabled={!selectedBookId || previewBatch.errorCount > 0 || hasMissingMemberMappings || stage === 'applying' || stage === 'applied'}
              onClick={handleApplyImport}
            >
              {stage === 'applying' ? t('imports.stage.applying') : t('imports.stage.apply')}
            </button>
            {hasMissingMemberMappings ? (
              <p className="mobileImportMessage mobileImportMessageWarning">
                <AlertCircle size={18} />
                {t('imports.memberMapping.missing')}
              </p>
            ) : null}
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

// BookDestination receives book state and renders destination selection plus a create-book path.
function BookDestination({
  books,
  isCreatingBook,
  newBookName,
  onCreateBook,
  onNewBookNameChange,
  onSelectBook,
  selectedBookId,
  suggestedBookName,
}: {
  books: BookListItem[];
  isCreatingBook: boolean;
  newBookName: string;
  onCreateBook: () => void;
  onNewBookNameChange: (name: string) => void;
  onSelectBook: (bookID: string) => void;
  selectedBookId: string;
  suggestedBookName: string;
}) {
  const { t } = useTranslation();
  const createName = newBookName.trim() || suggestedBookName.trim();

  return (
    <div className="mobileImportDestination" aria-label={t('imports.a11y.destinationBook')}>
      <div className="mobileImportSectionHeader">
        <span>{t('imports.destination.title')}</span>
        <strong>{books.find((book) => book.id === selectedBookId)?.name ?? t('imports.destination.none')}</strong>
      </div>
      <label>
        <span>{t('imports.destination.select')}</span>
        <select value={selectedBookId} disabled={!books.length} onChange={(event) => onSelectBook(event.target.value)}>
          {books.map((book) => (
            <option key={book.id} value={book.id}>{book.name}</option>
          ))}
        </select>
      </label>
      <div className="mobileImportCreateBook">
        <label>
          <span>{t('imports.destination.newBook')}</span>
          <input
            type="text"
            value={newBookName}
            placeholder={suggestedBookName || t('imports.destination.placeholder')}
            onChange={(event) => onNewBookNameChange(event.target.value)}
          />
        </label>
        <button
          className="mobileSecondaryButton"
          type="button"
          disabled={!createName || isCreatingBook}
          onClick={onCreateBook}
        >
          {isCreatingBook ? t('imports.destination.creating') : t('imports.destination.create')}
        </button>
      </div>
    </div>
  );
}

// MemberMappings receives required source members and renders uid/email mapping controls.
function MemberMappings({
  mappings,
  onChange,
  requirements,
}: {
  mappings: Record<string, string>;
  onChange: (sourceName: string, value: string) => void;
  requirements: MemberMappingRequirement[];
}) {
  const { t } = useTranslation();

  if (!requirements.length) {
    return null;
  }

  return (
    <div className="mobileImportMemberMappings" aria-label={t('imports.a11y.memberMappings')}>
      <div className="mobileImportSectionHeader">
        <span>{t('imports.memberMapping.title')}</span>
        <strong>{t('imports.memberMapping.required', { value: requirements.length })}</strong>
      </div>
      <p>{t('imports.memberMapping.description')}</p>
      <div className="mobileImportMappingRows">
        {requirements.map((requirement) => (
          <label key={requirement.sourceName} className="mobileImportMappingRow">
            <span>{requirement.sourceName}</span>
            <input
              type="text"
              autoCapitalize="none"
              autoCorrect="off"
              value={mappings[requirement.sourceName] ?? ''}
              placeholder={requirement.suggestedUserID || t('imports.memberMapping.placeholder')}
              aria-label={t('imports.memberMapping.userReference', { name: requirement.sourceName })}
              onChange={(event) => onChange(requirement.sourceName, event.target.value)}
            />
            {requirement.suggestedUserID ? (
              <small>{t('imports.memberMapping.matched', { value: requirement.suggestedUserID })}</small>
            ) : null}
          </label>
        ))}
      </div>
    </div>
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
    { key: 'members', label: t('imports.detected.members'), values: batch.detected.members },
    { key: 'merchants', label: t('imports.detected.merchants'), values: batch.detected.merchants },
    { key: 'tags', label: t('imports.detected.tags'), values: batch.detected.tags },
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
      {rows.map((row) => {
        const context = previewRowContext(row);
        return (
          <li key={row.rowNumber}>
            <div>
              <span>{t('imports.rowNumber', { value: row.rowNumber })}</span>
              <strong>{previewRowTitle(row, t('imports.unmappedRow'))}</strong>
            </div>
            <b>{row.amount ? `${row.currency ?? ''} ${row.amount}`.trim() : t('imports.none')}</b>
            {context ? <p>{context}</p> : null}
            {row.errors?.length || row.warnings?.length ? (
              <small>{[...(row.errors ?? []), ...(row.warnings ?? [])].join('; ')}</small>
            ) : null}
          </li>
        );
      })}
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

// useMemberMappingRequirements receives preview data and returns non-self source members that need resolution.
function useMemberMappingRequirements(batch: ImportPreviewBatch | null, actor: AuthActor, members: BookMember[]): MemberMappingRequirement[] {
  return useMemo(() => {
    if (!batch) {
      return [];
    }

    return memberMappingRequirements(batch, actor, members);
  }, [actor, batch, members]);
}

// useEffectiveMemberMappings receives requirements and typed values, then returns the mappings to submit.
function useEffectiveMemberMappings(requirements: MemberMappingRequirement[], mappings: Record<string, string>): Record<string, string> {
  return useMemo(() => {
    const effectiveMappings: Record<string, string> = {};
    for (const requirement of requirements) {
      const userReference = mappings[requirement.sourceName]?.trim() || requirement.suggestedUserID.trim();
      if (userReference) {
        effectiveMappings[requirement.sourceName] = userReference;
      }
    }

    return effectiveMappings;
  }, [mappings, requirements]);
}

// defaultMemberMappings receives preview data and returns known member-name matches.
function defaultMemberMappings(batch: ImportPreviewBatch, actor: AuthActor, members: BookMember[]): Record<string, string> {
  return Object.fromEntries(
    memberMappingRequirements(batch, actor, members)
      .filter((requirement) => requirement.suggestedUserID)
      .map((requirement) => [requirement.sourceName, requirement.suggestedUserID]),
  );
}

// memberMappingRequirements receives preview data and returns unique non-self source members with optional known matches.
function memberMappingRequirements(batch: ImportPreviewBatch, actor: AuthActor, members: BookMember[]): MemberMappingRequirement[] {
  const names: string[] = [];
  for (const row of batch.rows) {
    if (row.errors?.length) {
      continue;
    }
    for (const sourceName of [row.member, ...(row.participants ?? [])]) {
      const normalized = normalizeImportMemberName(sourceName);
      if (!normalized || isSelfImportMember(normalized, actor) || names.some((name) => normalizeImportMemberName(name) === normalized)) {
        continue;
      }
      names.push(sourceName ?? '');
    }
  }

  return names.map((sourceName) => {
    const knownMember = members.find((member) => {
      const sourceKey = normalizeImportMemberName(sourceName);
      return normalizeImportMemberName(member.userId) === sourceKey || normalizeImportMemberName(member.displayName) === sourceKey;
    });

    return { sourceName, suggestedUserID: knownMember?.userId ?? '' };
  });
}

// isSelfImportMember reports whether a source member name identifies the current actor.
function isSelfImportMember(sourceName: string, actor: AuthActor): boolean {
  return ['self', 'me', 'myself', '自己', normalizeImportMemberName(actor.userId), normalizeImportMemberName(actor.email)].includes(sourceName);
}

// normalizeImportMemberName receives a source name and returns a case-insensitive mapping key.
function normalizeImportMemberName(sourceName?: string): string {
  return (sourceName ?? '').trim().toLowerCase();
}

// previewRowTitle receives a preview row and fallback copy and returns a compact row title.
function previewRowTitle(row: ImportPreviewBatch['rows'][number], fallback: string): string {
  return row.note || row.merchant || row.category || row.sourceType || row.type || fallback;
}

// previewRowContext receives a preview row and returns account/member context for review.
function previewRowContext(row: ImportPreviewBatch['rows'][number]): string {
  const parts = [
    [row.account, row.destinationAccount].filter(Boolean).join(' -> '),
    row.member,
    row.participants?.join(', '),
    row.attribute,
    row.tags?.join(', '),
  ].filter(Boolean);

  return parts.join(' | ');
}

// describeFile receives a selected file and returns a compact display name with size.
function describeFile(file: File, fallbackType: string): string {
  const size = Math.max(file.size / 1024, 1).toFixed(0);
  return `${file.type || fallbackType} - ${size} KB`;
}
