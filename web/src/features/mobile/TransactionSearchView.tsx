import { Search, X } from 'lucide-react';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { type Account, type BookMember, type Category, type Entry } from '@/lib/api/ledger';
import { formatMoney } from '@/lib/money';
import './transaction-search.css';

type TransactionSearchViewProps = {
  accounts: Account[];
  categories: Category[];
  entries: Entry[];
  isLoading: boolean;
  error: string;
  members?: BookMember[];
  onClose: () => void;
  onOpenEntry?: (entryId: string) => void;
  onQueryChange: (query: string) => void;
  query: string;
  title?: string;
};

// TransactionSearchView receives ledger entries and returns a searchable mobile transaction panel.
export function TransactionSearchView({
  accounts,
  categories,
  entries,
  error,
  isLoading,
  members = [],
  onClose,
  onOpenEntry,
  onQueryChange,
  query,
  title,
}: TransactionSearchViewProps) {
  const { t } = useTranslation();
  const results = useMemo(
    () => filterEntries(entries, accounts, categories, members, query),
    [accounts, categories, entries, members, query],
  );

  return (
    <section className="tabPanel transactionSearchPanel" aria-label={title ?? t('mobile.search.title')}>
      <div className="transactionSearchHeader">
        <div>
          <p>{t('mobile.search.eyebrow')}</p>
          <h1>{title ?? t('mobile.search.title')}</h1>
        </div>
        <button type="button" aria-label={t('mobile.search.close')} onClick={onClose}>
          <X size={22} />
        </button>
      </div>

      <label className="transactionSearchBox">
        <Search size={18} />
        <input
          autoFocus
          aria-label={t('mobile.search.input')}
          placeholder={t('mobile.search.placeholder')}
          value={query}
          onChange={(event) => onQueryChange(event.target.value)}
        />
      </label>

      {error ? <p className="mobileSearchMessage mobileSearchMessageError">{error}</p> : null}
      {isLoading ? <p className="mobileSearchMessage">{t('mobile.search.loading')}</p> : null}

      {!isLoading && !error ? (
        results.length ? (
          <ul className="transactionSearchResults" aria-label={t('mobile.search.results')}>
            {results.map((entry) => {
              const title = entryTitle(entry, categories);
              const account = accounts.find((item) => item.id === entry.accountId);
              return (
                <li key={entry.id}>
                  <button
                    type="button"
                    className="transactionSearchResultButton"
                    aria-label={t('mobile.transactions.openEntry', { title })}
                    onClick={() => onOpenEntry?.(entry.id)}
                  >
                    <div>
                      <strong>{title}</strong>
                      <span>
                        {t('mobile.search.resultMeta', {
                          account: account?.name ?? t('mobile.transactions.accountFallback'),
                          time: formatEntryTime(entry.occurredAt),
                          type: entry.type,
                        })}
                      </span>
                    </div>
                    <b>{formatMoney(entry.amountCents, entry.transactionCurrency)}</b>
                  </button>
                </li>
              );
            })}
          </ul>
        ) : (
          <p className="emptyState">{query ? t('mobile.search.noResults') : t('mobile.search.empty')}</p>
        )
      ) : null}
    </section>
  );
}

// filterEntries receives entries, ledger dimensions, and a query, and returns matching entries.
function filterEntries(
  entries: Entry[],
  accounts: Account[],
  categories: Category[],
  members: BookMember[],
  query: string,
): Entry[] {
  const normalized = query.trim().toLowerCase();
  if (!normalized) {
    return sortEntries(entries);
  }

  return sortEntries(
    entries.filter((entry) => {
      const account = accounts.find((item) => item.id === entry.accountId);
      const destinationAccount = accounts.find((item) => item.id === entry.destinationAccountId);
      const category = categories.find((item) => item.id === entry.categoryId);
      const parentCategory = categories.find((item) => item.id === category?.parentId);
      const member = members.find((item) => item.userId === entry.creatorUserId);
      const haystack = [
        entry.note,
        entry.merchant,
        entry.type,
        entry.transactionCurrency,
        String(entry.amountCents / 100),
        formatMoney(entry.amountCents, entry.transactionCurrency),
        account?.name,
        destinationAccount?.name,
        parentCategory?.name,
        category?.name,
        member?.displayName,
        member?.userId,
        entry.creatorUserId,
        ...(entry.tags ?? []),
      ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();

      return haystack.includes(normalized);
    }),
  );
}

// entryTitle receives an entry and categories and returns a compact search result title.
function entryTitle(entry: Entry, categories: Category[]): string {
  const category = categories.find((item) => item.id === entry.categoryId);
  return entry.note || entry.merchant || category?.name || entry.type;
}

// formatEntryTime receives an ISO timestamp and returns compact UTC date text for search results.
function formatEntryTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return new Intl.DateTimeFormat('en-US', {
    month: 'short',
    day: '2-digit',
    timeZone: 'UTC',
  }).format(date);
}

// sortEntries receives entries and returns them newest first for search review.
function sortEntries(entries: Entry[]): Entry[] {
  return [...entries].sort((left, right) => new Date(right.occurredAt).getTime() - new Date(left.occurredAt).getTime());
}
