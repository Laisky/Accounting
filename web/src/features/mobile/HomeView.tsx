import { Package } from 'lucide-react';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { EmptyState } from '@/components/ui';
import { type Account, type Category, type Entry, type LedgerSummary } from '@/lib/api/ledger';
import { convertEntryAmountCents, formatMoney } from '@/lib/money';

type HomeViewProps = {
  accounts: Account[];
  bookName: string;
  categories: Category[];
  currencyCode: string;
  entries: Entry[];
  onOpenEntry?: (entryId: string) => void;
  onRecordEntry?: () => void;
  rateIndex: Map<string, number>;
  summary: LedgerSummary;
};

type EntryDayGroup = {
  id: string;
  label: string;
  incomeCents: number;
  expenseCents: number;
  entries: Entry[];
};

// HomeView receives the current ledger snapshot and returns the mobile transaction feed.
export function HomeView({
  accounts,
  bookName,
  categories,
  currencyCode,
  entries,
  onOpenEntry,
  onRecordEntry,
  rateIndex,
  summary,
}: HomeViewProps) {
  const { t } = useTranslation();
  const groups = useMemo(() => groupEntriesByDay(entries, currencyCode, rateIndex), [currencyCode, entries, rateIndex]);
  const monthlyExpenseCents = useMemo(
    () => currentMonthExpenseCents(entries, currencyCode, rateIndex),
    [currencyCode, entries, rateIndex],
  );
  const summaryBalanceCents = convertSummaryBalanceCents(summary, currencyCode, rateIndex);
  const budgetTotalCents = Math.max(monthlyExpenseCents, summaryBalanceCents, 0);
  const remainingCents = Math.max(0, budgetTotalCents - monthlyExpenseCents);
  const progress = budgetTotalCents > 0 ? Math.min(100, Math.round((monthlyExpenseCents / budgetTotalCents) * 100)) : 0;

  return (
    <section className="tabPanel homePanel" aria-label={t('mobile.home.title')}>
      <section className="budgetCard" aria-label={t('mobile.budget.title')}>
        <div>
          <h1>{t('mobile.budget.title')}</h1>
          <p>
            {t('mobile.budget.remaining')} <strong>{formatMoney(remainingCents, currencyCode)}</strong>
          </p>
        </div>
        <div
          className="budgetTrack"
          role="meter"
          aria-label={t('mobile.budget.title')}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={progress}
          aria-valuetext={t('mobile.budget.meter', {
            spent: formatMoney(monthlyExpenseCents, currencyCode),
            total: formatMoney(budgetTotalCents, currencyCode),
            percent: progress,
          })}
        >
          <span style={{ width: `${progress}%` }} />
        </div>
        <footer>
          <span>{t('mobile.budget.total', { amount: formatMoney(budgetTotalCents, currencyCode) })}</span>
          <span>{t('mobile.budget.spent', { amount: formatMoney(monthlyExpenseCents, currencyCode) })}</span>
          <b>{progress}%</b>
        </footer>
      </section>

      <section className="transactionList" aria-label={t('mobile.transactions.title')}>
        {groups.length ? (
          groups.map((group) => (
            <article className="dayGroup" key={group.id}>
              <header>
                <span>{group.label}</span>
                <b>
                  {t('mobile.transactions.dayTotals', {
                    income: formatMoney(group.incomeCents, currencyCode),
                    expense: formatMoney(group.expenseCents, currencyCode),
                  })}
                </b>
              </header>
              <ul>
                {group.entries.map((entry) => {
                  const category = categories.find((item) => item.id === entry.categoryId);
                  const account = accounts.find((item) => item.id === entry.accountId);
                  const title = entry.note || entry.merchant || category?.name || t('mobile.transactions.category');
                  const signedAmount = entry.type === 'expense' ? -entry.amountCents : entry.amountCents;

                  return (
                    <li key={entry.id}>
                      <button
                        type="button"
                        className="transactionRowButton"
                        aria-label={t('mobile.transactions.openEntry', { title })}
                        onClick={() => onOpenEntry?.(entry.id)}
                      >
                        <span className="transactionIcon">
                          <Package size={20} />
                        </span>
                        <div>
                          <strong>{title}</strong>
                          <small>
                            {t('mobile.transactions.entryMeta', {
                              time: formatEntryTime(entry.occurredAt),
                              account: account?.name ?? t('mobile.transactions.accountFallback'),
                              book: bookName,
                            })}
                          </small>
                        </div>
                        <b>{formatMoney(signedAmount, entry.transactionCurrency || currencyCode)}</b>
                      </button>
                    </li>
                  );
                })}
              </ul>
            </article>
          ))
        ) : (
          <EmptyState
            icon={<Package size={22} />}
            title={t('mobile.home.emptyTitle')}
            description={t('mobile.home.emptyBody')}
            actionLabel={onRecordEntry ? t('mobile.home.recordCta') : undefined}
            onAction={onRecordEntry}
          />
        )}
      </section>
    </section>
  );
}

// currentMonthExpenseCents receives entries and returns expense cents for the current UTC month.
function currentMonthExpenseCents(entries: Entry[], currencyCode: string, rates: Map<string, number>): number {
  const now = new Date();
  const year = now.getUTCFullYear();
  const month = now.getUTCMonth();

  return entries.reduce((sum, entry) => {
    const occurredAt = new Date(entry.occurredAt);
    if (entry.type !== 'expense' || occurredAt.getUTCFullYear() !== year || occurredAt.getUTCMonth() !== month) {
      return sum;
    }

    return sum + (convertEntryAmountCents(entry, currencyCode, rates) ?? 0);
  }, 0);
}

// formatEntryTime receives an ISO timestamp and returns a compact UTC time label.
function formatEntryTime(value: string): string {
  return new Intl.DateTimeFormat('en-US', {
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
    timeZone: 'UTC',
  }).format(new Date(value));
}

// formatGroupDate receives an ISO date key and returns the transaction group label.
function formatGroupDate(value: string): string {
  return new Intl.DateTimeFormat('en-US', {
    month: '2-digit',
    day: '2-digit',
    weekday: 'short',
    timeZone: 'UTC',
  }).format(new Date(`${value}T00:00:00Z`));
}

// groupEntriesByDay receives entries and returns descending UTC day groups with daily totals.
function groupEntriesByDay(entries: Entry[], currencyCode: string, rates: Map<string, number>): EntryDayGroup[] {
  const groups = new Map<string, EntryDayGroup>();
  const sorted = [...entries].sort(
    (left, right) => new Date(right.occurredAt).getTime() - new Date(left.occurredAt).getTime(),
  );

  for (const entry of sorted) {
    const id = entry.occurredAt.slice(0, 10);
    const group = groups.get(id) ?? {
      id,
      label: formatGroupDate(id),
      incomeCents: 0,
      expenseCents: 0,
      entries: [],
    };
    const amountCents = convertEntryAmountCents(entry, currencyCode, rates) ?? 0;
    if (entry.type === 'income') {
      group.incomeCents += amountCents;
    } else if (entry.type === 'expense') {
      group.expenseCents += amountCents;
    }
    group.entries.push(entry);
    groups.set(id, group);
  }

  return [...groups.values()];
}

// convertSummaryBalanceCents receives the API summary and returns balance cents in the display currency when rates are available.
function convertSummaryBalanceCents(summary: LedgerSummary, currencyCode: string, rates: Map<string, number>): number {
  if (summary.currency === currencyCode) {
    return summary.balanceCents;
  }

  const sourceRate = rates.get(summary.currency);
  const targetRate = rates.get(currencyCode);
  if (!sourceRate || !targetRate) {
    return 0;
  }

  return Math.round((summary.balanceCents * targetRate) / sourceRate);
}
