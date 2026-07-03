import { Package } from 'lucide-react';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { type Account, type Category, type Entry, type LedgerSummary } from '../../lib/api/ledger';
import { formatMoney } from '../../lib/money';

type HomeViewProps = {
  accounts: Account[];
  bookName: string;
  categories: Category[];
  currencyCode: string;
  entries: Entry[];
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
export function HomeView({ accounts, bookName, categories, currencyCode, entries, summary }: HomeViewProps) {
  const { t } = useTranslation();
  const groups = useMemo(() => groupEntriesByDay(entries), [entries]);
  const monthlyExpenseCents = useMemo(() => currentMonthExpenseCents(entries), [entries]);
  const budgetTotalCents = Math.max(monthlyExpenseCents, summary.balanceCents, 1);
  const remainingCents = Math.max(0, budgetTotalCents - monthlyExpenseCents);
  const progress = Math.min(100, Math.round((monthlyExpenseCents / budgetTotalCents) * 100));

  return (
    <section className="tabPanel homePanel" aria-label={t('mobile.home.title')}>
      <section className="budgetCard" aria-label={t('mobile.budget.title')}>
        <div>
          <h1>{t('mobile.budget.title')}</h1>
          <p>
            {t('mobile.budget.remaining')} <strong>{formatMoney(remainingCents, currencyCode)}</strong>
          </p>
        </div>
        <div className="budgetTrack" aria-hidden="true">
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
                <b>{t('mobile.transactions.dayTotals', { income: formatMoney(group.incomeCents, currencyCode), expense: formatMoney(group.expenseCents, currencyCode) })}</b>
              </header>
              <ul>
                {group.entries.map((entry) => {
                  const category = categories.find((item) => item.id === entry.categoryId);
                  const account = accounts.find((item) => item.id === entry.accountId);
                  const title = entry.note || entry.merchant || category?.name || t('mobile.transactions.category');
                  const signedAmount = entry.type === 'expense' ? -entry.amountCents : entry.amountCents;

                  return (
                    <li key={entry.id} aria-label={t('mobile.transactions.entryLabel', { title })}>
                      <span className="transactionIcon">
                        <Package size={20} />
                      </span>
                      <div>
                        <strong>{title}</strong>
                        <small>{t('mobile.transactions.entryMeta', { time: formatEntryTime(entry.occurredAt), account: account?.name ?? t('mobile.transactions.accountFallback'), book: bookName })}</small>
                      </div>
                      <b>{formatMoney(signedAmount, entry.transactionCurrency || currencyCode)}</b>
                    </li>
                  );
                })}
              </ul>
            </article>
          ))
        ) : (
          <p className="emptyState">{t('mobile.transactions.empty')}</p>
        )}
      </section>
    </section>
  );
}

// currentMonthExpenseCents receives entries and returns expense cents for the current UTC month.
function currentMonthExpenseCents(entries: Entry[]): number {
  const now = new Date();
  const year = now.getUTCFullYear();
  const month = now.getUTCMonth();

  return entries.reduce((sum, entry) => {
    const occurredAt = new Date(entry.occurredAt);
    if (entry.type !== 'expense' || occurredAt.getUTCFullYear() !== year || occurredAt.getUTCMonth() !== month) {
      return sum;
    }

    return sum + entry.amountCents;
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
function groupEntriesByDay(entries: Entry[]): EntryDayGroup[] {
  const groups = new Map<string, EntryDayGroup>();
  const sorted = [...entries].sort((left, right) => new Date(right.occurredAt).getTime() - new Date(left.occurredAt).getTime());

  for (const entry of sorted) {
    const id = entry.occurredAt.slice(0, 10);
    const group = groups.get(id) ?? {
      id,
      label: formatGroupDate(id),
      incomeCents: 0,
      expenseCents: 0,
      entries: [],
    };
    if (entry.type === 'income') {
      group.incomeCents += entry.amountCents;
    } else if (entry.type === 'expense') {
      group.expenseCents += entry.amountCents;
    }
    group.entries.push(entry);
    groups.set(id, group);
  }

  return [...groups.values()];
}
