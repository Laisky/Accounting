import {
  Activity,
  AlertCircle,
  CheckCircle2,
  Database,
  FileSpreadsheet,
  HardDriveUpload,
  Landmark,
  ReceiptText,
  UploadCloud,
  WalletCards,
} from 'lucide-react';
import { type ChangeEvent, type DragEvent, type ReactNode, useEffect, useMemo, useState } from 'react';

type Summary = {
  balanceCents: number;
  currency: string;
  entryCount: number;
};

type ImportSource = {
  id: string;
  label: string;
  description: string;
  acceptedFormats: string;
  enabled: boolean;
};

type ImportStage = 'empty' | 'ready' | 'staged';

const currency = new Intl.NumberFormat('en-US', {
  style: 'currency',
  currency: 'USD',
});

const importSources: ImportSource[] = [
  {
    id: 'wacai',
    label: 'Wacai',
    description: 'Spreadsheet exports from Wacai books, accounts, and bill records.',
    acceptedFormats: '.xlsx, .xls, .csv',
    enabled: true,
  },
];

// App renders the main accounting workspace, accepts no parameters, and returns the first import flow.
export function App() {
  const summary = useSummary();
  const [selectedSourceID, setSelectedSourceID] = useState(importSources[0].id);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [stage, setStage] = useState<ImportStage>('empty');
  const selectedSource = importSources.find((source) => source.id === selectedSourceID) ?? importSources[0];
  const balance = currency.format((summary?.balanceCents ?? 0) / 100);
  const fileSummary = useMemo(() => describeImportFile(selectedFile), [selectedFile]);
  const rowEstimate = selectedFile ? estimateRows(selectedFile.size) : 0;

  // handleFileChange receives a file input event, stores the first selected file, and returns no value.
  function handleFileChange(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0] ?? null;
    setSelectedFile(file);
    setStage(file ? 'ready' : 'empty');
  }

  // handleDrop receives a drag event, stores the first dropped spreadsheet file, and returns no value.
  function handleDrop(event: DragEvent<HTMLLabelElement>) {
    event.preventDefault();
    const file = event.dataTransfer.files[0] ?? null;
    setSelectedFile(file);
    setStage(file ? 'ready' : 'empty');
  }

  // handleStageImport accepts no parameters, marks the file as ready for the future backend upload endpoint, and returns no value.
  function handleStageImport() {
    if (!selectedFile) {
      return;
    }

    setStage('staged');
  }

  return (
    <main className="shell">
      <section className="workspace">
        <header className="masthead">
          <div>
            <p className="eyebrow">Accounting workspace</p>
            <h1>Import Wacai books into a clean ledger.</h1>
          </div>
          <button className="iconButton" type="button" aria-label="Open activity">
            <Activity size={20} />
          </button>
        </header>

        <section className="summaryBand" aria-label="Ledger summary">
          <div className="summaryItem summaryBalance">
            <span>Current balance</span>
            <strong>{balance}</strong>
          </div>
          <div className="summaryItem">
            <span>Currency</span>
            <strong>{summary?.currency ?? 'USD'}</strong>
          </div>
          <div className="summaryItem">
            <span>Entries</span>
            <strong>{summary?.entryCount ?? 0}</strong>
          </div>
        </section>

        <section className="importLayout" aria-label="Import data">
          <div className="importMain">
            <div className="sectionHeading">
              <div>
                <p className="eyebrow">Data import</p>
                <h2>Choose the source, upload the export, review before commit.</h2>
              </div>
              <span className="sourceBadge">PostgreSQL + MinIO ready</span>
            </div>

            <div className="sourceField">
              <label htmlFor="source">Data source</label>
              <select
                id="source"
                value={selectedSourceID}
                onChange={(event) => setSelectedSourceID(event.target.value)}
              >
                {importSources.map((source) => (
                  <option key={source.id} disabled={!source.enabled} value={source.id}>
                    {source.label}
                  </option>
                ))}
              </select>
              <p>{selectedSource.description}</p>
            </div>

            <label
              className={`dropZone ${selectedFile ? 'dropZoneReady' : ''}`}
              onDragOver={(event) => event.preventDefault()}
              onDrop={handleDrop}
            >
              <input
                type="file"
                accept={selectedSource.acceptedFormats}
                aria-label="Upload Wacai export file"
                onChange={handleFileChange}
              />
              <UploadCloud size={34} />
              <span>{selectedFile ? selectedFile.name : 'Upload a Wacai export file'}</span>
              <small>{selectedFile ? fileSummary : 'Accepted formats: XLSX, XLS, CSV'}</small>
            </label>

            <div className="actionRow">
              <button className="primaryButton" type="button" disabled={!selectedFile} onClick={handleStageImport}>
                <HardDriveUpload size={18} />
                <span>{stage === 'staged' ? 'Import staged' : 'Stage import'}</span>
              </button>
              <button
                className="ghostButton"
                type="button"
                disabled={!selectedFile}
                onClick={() => {
                  setSelectedFile(null);
                  setStage('empty');
                }}
              >
                Clear file
              </button>
            </div>
          </div>

          <aside className="reviewPanel" aria-label="Import review">
            <div className="reviewHeader">
              <FileSpreadsheet size={22} />
              <div>
                <h3>Review queue</h3>
                <p>{stage === 'empty' ? 'No file selected' : `${selectedSource.label} export detected`}</p>
              </div>
            </div>

            <dl className="fileFacts">
              <div>
                <dt>Source</dt>
                <dd>{selectedSource.label}</dd>
              </div>
              <div>
                <dt>Rows</dt>
                <dd>{rowEstimate ? `~${rowEstimate}` : 'Pending'}</dd>
              </div>
              <div>
                <dt>Storage</dt>
                <dd>MinIO object</dd>
              </div>
              <div>
                <dt>Metadata</dt>
                <dd>PostgreSQL batch</dd>
              </div>
            </dl>

            <ol className="pipeline">
              <PipelineStep
                active={stage !== 'empty'}
                icon={<CheckCircle2 size={18} />}
                label="Validate source format"
              />
              <PipelineStep
                active={stage === 'staged'}
                icon={<HardDriveUpload size={18} />}
                label="Store raw file in MinIO"
              />
              <PipelineStep
                active={stage === 'staged'}
                icon={<Database size={18} />}
                label="Create import batch in PostgreSQL"
              />
              <PipelineStep
                active={false}
                icon={<AlertCircle size={18} />}
                label="Map categories and accounts"
              />
            </ol>
          </aside>
        </section>

        <section className="quickGrid" aria-label="Primary workflows">
          <button type="button">
            <ReceiptText size={18} />
            <span>New entry</span>
          </button>
          <button type="button">
            <WalletCards size={18} />
            <span>Accounts</span>
          </button>
          <button type="button">
            <Landmark size={18} />
            <span>Reconcile</span>
          </button>
        </section>
      </section>
    </main>
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

// useSummary accepts no parameters, loads the current ledger summary, and returns a fallback on request failure.
function useSummary(): Summary | null {
  const [summary, setSummary] = useState<Summary | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    fetch('/api/ledger/summary', { signal: controller.signal })
      .then((response) => {
        if (!response.ok) {
          throw new Error(`summary request failed: ${response.status}`);
        }
        return response.json() as Promise<Summary>;
      })
      .then(setSummary)
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === 'AbortError') {
          return;
        }
        setSummary({ balanceCents: 0, currency: 'USD', entryCount: 0 });
      });

    return () => controller.abort();
  }, []);

  return summary;
}

// describeImportFile receives a selected file and returns formatted upload summary text.
function describeImportFile(file: File | null): string {
  if (!file) {
    return 'No file selected';
  }

  const sizeInMB = file.size / 1024 / 1024;
  return `${file.type || 'Spreadsheet'} - ${sizeInMB.toFixed(2)} MB`;
}

// estimateRows receives a file size in bytes and returns a conservative row estimate before parsing.
function estimateRows(size: number): number {
  if (size <= 0) {
    return 1;
  }

  return Math.max(12, Math.round(size / 180));
}
