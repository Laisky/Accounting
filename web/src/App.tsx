import { Activity, Landmark, ReceiptText, WalletCards } from 'lucide-react';
import { useEffect, useState } from 'react';

type Summary = {
  balanceCents: number;
  currency: string;
  entryCount: number;
};

const currency = new Intl.NumberFormat('en-US', {
  style: 'currency',
  currency: 'USD',
});

export function App() {
  const summary = useSummary();
  const balance = currency.format((summary?.balanceCents ?? 0) / 100);

  return (
    <main className="shell">
      <section className="workspace">
        <header className="masthead">
          <div>
            <p className="eyebrow">Accounting workspace</p>
            <h1>Daily money movement, ready for real books.</h1>
          </div>
          <button className="iconButton" type="button" aria-label="Open activity">
            <Activity size={20} />
          </button>
        </header>

        <section className="balancePanel" aria-label="Ledger summary">
          <div className="balanceCopy">
            <span>Current balance</span>
            <strong>{balance}</strong>
          </div>
          <dl>
            <div>
              <dt>Currency</dt>
              <dd>{summary?.currency ?? 'USD'}</dd>
            </div>
            <div>
              <dt>Entries</dt>
              <dd>{summary?.entryCount ?? 0}</dd>
            </div>
          </dl>
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
