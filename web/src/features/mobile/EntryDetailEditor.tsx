import { Pencil, Trash2 } from 'lucide-react';
import { type FormEvent, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { type Account, type Category, type Entry, type EntryUpdateInput } from '@/lib/api/ledger';
import { formatMoney, supportedCurrencies } from '@/lib/money';
import './entry-detail-editor.css';

type EntryDetailEditorProps = {
  account?: Account;
  accounts: Account[];
  categories: Category[];
  entry: Entry;
  isBusy: boolean;
  openSignal?: number;
  onDeleteEntry: (entryId: string) => Promise<void>;
  onUpdateEntry: (entryId: string, input: EntryUpdateInput) => Promise<void>;
};

// EntryDetailEditor receives one entry and returns an expandable transaction correction form.
export function EntryDetailEditor({
  account,
  accounts,
  categories,
  entry,
  isBusy,
  openSignal,
  onDeleteEntry,
  onUpdateEntry,
}: EntryDetailEditorProps) {
  const { t } = useTranslation();
  const [isOpen, setIsOpen] = useState(false);
  const [draft, setDraft] = useState(() => buildDraft(entry));
  const title = entryTitle(entry, categories);
  const canSave = draft.amountCents > 0 && Boolean(draft.accountId);

  useEffect(() => {
    if (openSignal) {
      queueMicrotask(() => setIsOpen(true));
    }
  }, [openSignal]);

  // handleSubmit receives a form event and patches editable transaction details.
  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!canSave) {
      return;
    }

    await onUpdateEntry(entry.id, {
      accountId: draft.accountId,
      amountCents: draft.amountCents,
      categoryId: draft.categoryId || undefined,
      merchant: draft.merchant.trim(),
      note: draft.note.trim(),
      occurredAt: toSafeISOString(draft.occurredAt),
      tags: parseTags(draft.tags),
      transactionCurrency: draft.currency,
    });
  }

  return (
    <li aria-label={t('mobile.transactions.entryLabel', { title })}>
      <div className="recordRecentSummary">
        <span className="transactionIcon">
          <Pencil size={20} />
        </span>
        <div>
          <strong>{entry.note || entry.merchant || title}</strong>
          <small>
            {t('mobile.transactions.entryMeta', {
              time: formatEntryTime(entry.occurredAt),
              account: account?.name ?? t('mobile.transactions.accountFallback'),
              book: entry.bookId.slice(0, 8),
            })}
          </small>
        </div>
        <b>{formatMoney(entry.amountCents, entry.transactionCurrency)}</b>
      </div>
      <div className="recordRecentActions">
        <button
          className="mobileSecondaryButton"
          type="button"
          aria-expanded={isOpen}
          onClick={() => setIsOpen((current) => !current)}
        >
          {t('mobile.transactions.editDetails')}
        </button>
        <button className="mobileDangerButton" type="button" disabled={isBusy} onClick={() => onDeleteEntry(entry.id)}>
          <Trash2 size={16} />
          {t('mobile.transactions.deleteEntry')}
        </button>
      </div>
      {isOpen ? (
        <form
          className="entryDetailForm"
          aria-label={t('mobile.transactions.editFor', { title })}
          onSubmit={handleSubmit}
        >
          <label>
            <span>{t('common.amount')}</span>
            <input
              aria-label={t('mobile.transactions.amountFor', { title })}
              inputMode="decimal"
              value={draft.amount}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  amount: event.target.value,
                  amountCents: parseAmountToCents(event.target.value),
                }))
              }
            />
          </label>
          <label>
            <span>{t('mobile.record.time')}</span>
            <input
              aria-label={t('mobile.transactions.timeFor', { title })}
              type="datetime-local"
              value={draft.occurredAt}
              onChange={(event) => setDraft((current) => ({ ...current, occurredAt: event.target.value }))}
            />
          </label>
          <label>
            <span>{t('mobile.record.account')}</span>
            <select
              aria-label={t('mobile.transactions.accountFor', { title })}
              value={draft.accountId}
              onChange={(event) => setDraft((current) => ({ ...current, accountId: event.target.value }))}
            >
              {accounts.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            <span>{t('mobile.transactions.category')}</span>
            <select
              aria-label={t('mobile.transactions.categoryFor', { title })}
              value={draft.categoryId}
              onChange={(event) => setDraft((current) => ({ ...current, categoryId: event.target.value }))}
            >
              <option value="">{t('mobile.record.categoryFallback')}</option>
              {categories
                .filter((category) => !category.archived)
                .map((category) => (
                  <option key={category.id} value={category.id}>
                    {category.name}
                  </option>
                ))}
            </select>
          </label>
          <label>
            <span>{t('mobile.record.currency')}</span>
            <select
              aria-label={t('mobile.transactions.currencyFor', { title })}
              value={draft.currency}
              onChange={(event) => setDraft((current) => ({ ...current, currency: event.target.value }))}
            >
              {supportedCurrencies.map((currency) => (
                <option key={currency} value={currency}>
                  {currency}
                </option>
              ))}
            </select>
          </label>
          <label>
            <span>{t('mobile.transactions.merchant')}</span>
            <input
              aria-label={t('mobile.transactions.merchantFor', { title })}
              value={draft.merchant}
              onChange={(event) => setDraft((current) => ({ ...current, merchant: event.target.value }))}
            />
          </label>
          <label>
            <span>{t('mobile.transactions.tags')}</span>
            <input
              aria-label={t('mobile.transactions.tagsFor', { title })}
              value={draft.tags}
              onChange={(event) => setDraft((current) => ({ ...current, tags: event.target.value }))}
            />
          </label>
          <label>
            <span>{t('common.note')}</span>
            <input
              aria-label={t('mobile.transactions.noteFor', { title })}
              value={draft.note}
              onChange={(event) => setDraft((current) => ({ ...current, note: event.target.value }))}
            />
          </label>
          <button className="mobilePrimaryButton" type="submit" disabled={isBusy || !canSave}>
            {t('mobile.transactions.saveDetails')}
          </button>
        </form>
      ) : null}
    </li>
  );
}

// buildDraft receives an entry and returns editable form state.
function buildDraft(entry: Entry) {
  return {
    accountId: entry.accountId ?? '',
    amount: centsToAmountInput(entry.amountCents),
    amountCents: entry.amountCents,
    categoryId: entry.categoryId ?? '',
    currency: entry.transactionCurrency || entry.bookReportingCurrency || 'USD',
    merchant: entry.merchant ?? '',
    note: entry.note ?? '',
    occurredAt: toDateTimeLocalValue(new Date(entry.occurredAt)),
    tags: entry.tags?.join(', ') ?? '',
  };
}

// centsToAmountInput receives cents and returns decimal input text.
function centsToAmountInput(cents: number): string {
  return String(Math.round(cents) / 100);
}

// entryTitle receives an entry and categories and returns a stable display title.
function entryTitle(entry: Entry, categories: Category[]): string {
  const category = categories.find((item) => item.id === entry.categoryId);
  return entry.note || entry.merchant || category?.name || entry.type;
}

// formatEntryTime receives an ISO timestamp and returns compact UTC time text.
function formatEntryTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return new Intl.DateTimeFormat('en-US', {
    month: 'short',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
    timeZone: 'UTC',
  }).format(date);
}

// parseAmountToCents receives decimal amount text and returns whole cents.
function parseAmountToCents(value: string): number {
  const parsed = Number(value.trim());
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return 0;
  }

  return Math.round(parsed * 100);
}

// parseTags receives comma-separated text and returns normalized tag names.
function parseTags(value: string): string[] {
  return value
    .split(',')
    .map((tag) => tag.trim())
    .filter(Boolean)
    .slice(0, 20);
}

// toDateTimeLocalValue receives a date and returns a datetime-local input value.
function toDateTimeLocalValue(value: Date): string {
  if (Number.isNaN(value.getTime())) {
    return toDateTimeLocalValue(new Date());
  }

  const year = value.getUTCFullYear();
  const month = String(value.getUTCMonth() + 1).padStart(2, '0');
  const day = String(value.getUTCDate()).padStart(2, '0');
  const hour = String(value.getUTCHours()).padStart(2, '0');
  const minute = String(value.getUTCMinutes()).padStart(2, '0');
  return `${year}-${month}-${day}T${hour}:${minute}`;
}

// toSafeISOString receives a datetime-local value and returns a safe ISO timestamp.
function toSafeISOString(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return new Date().toISOString();
  }

  return date.toISOString();
}
