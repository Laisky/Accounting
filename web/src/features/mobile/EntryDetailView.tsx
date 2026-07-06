import { CalendarDays } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import {
  type Account,
  type BookListItem,
  type BookMember,
  type Category,
  type Entry,
  type EntryUpdateInput,
} from '@/lib/api/ledger';
import { formatMoney } from '@/lib/money';
import { EntryDetailEditor } from './EntryDetailEditor';
import './entry-detail.css';

type EntryDetailViewProps = {
  accounts: Account[];
  books: BookListItem[];
  categories: Category[];
  entry?: Entry;
  editorOpenSignal?: number;
  error?: string;
  isLoading: boolean;
  isSaving: boolean;
  members: BookMember[];
  onDeleteEntry: (entryId: string) => Promise<void>;
  onUpdateEntry: (entryId: string, input: EntryUpdateInput) => Promise<void>;
};

type DetailRow = {
  label: string;
  value: string;
};

// EntryDetailView receives one ledger entry and returns the mobile bill detail page.
export function EntryDetailView({
  accounts,
  books,
  categories,
  editorOpenSignal,
  entry,
  error,
  isLoading,
  isSaving,
  members,
  onDeleteEntry,
  onUpdateEntry,
}: EntryDetailViewProps) {
  const { t } = useTranslation();

  if (isLoading) {
    return (
      <section className="tabPanel entryDetailPanel" aria-label={t('mobile.entryDetail.title')}>
        <p className="mobileSearchMessage">{t('mobile.entryDetail.loading')}</p>
      </section>
    );
  }

  if (!entry) {
    return (
      <section className="tabPanel entryDetailPanel" aria-label={t('mobile.entryDetail.title')}>
        {error ? <p className="mobileInlineError">{error}</p> : null}
        <p className="emptyState">{t('mobile.entryDetail.notFound')}</p>
      </section>
    );
  }

  const account = accounts.find((item) => item.id === entry.accountId);
  const category = categories.find((item) => item.id === entry.categoryId);
  const parentCategory = categories.find((item) => item.id === category?.parentId);
  const book = books.find((item) => item.id === entry.bookId);
  const title = entryTitle(entry, category, parentCategory, t('mobile.transactions.category'));
  const signedAmount = signedEntryAmount(entry);
  const rows: DetailRow[] = [
    { label: t('mobile.entryDetail.time'), value: formatDetailDate(entry.occurredAt) },
    { label: t('mobile.entryDetail.account'), value: account?.name ?? t('mobile.transactions.accountFallback') },
    { label: t('mobile.entryDetail.member'), value: memberName(entry.creatorUserId, members) },
    { label: t('mobile.entryDetail.merchant'), value: entry.merchant || t('mobile.entryDetail.noMerchant') },
    { label: t('mobile.entryDetail.reimbursement'), value: t('mobile.entryDetail.notReimbursed') },
  ];
  const auditRows: DetailRow[] = [
    { label: t('mobile.entryDetail.book'), value: book?.name ?? entry.bookId },
    { label: t('mobile.entryDetail.creator'), value: memberName(entry.creatorUserId, members) },
    { label: t('mobile.entryDetail.recordedAt'), value: formatDetailDate(entry.createdAt) },
  ];

  return (
    <section className="tabPanel entryDetailPanel" aria-label={t('mobile.entryDetail.label', { title })}>
      <section className="entryDetailHero" aria-label={t('mobile.entryDetail.summary')}>
        <span className="transactionIcon">
          <CalendarDays size={21} />
        </span>
        <div>
          <strong>{title}</strong>
          <small>{categoryPath(category, parentCategory) || t('mobile.transactions.category')}</small>
        </div>
        <b>{formatMoney(signedAmount, entry.transactionCurrency || entry.bookReportingCurrency)}</b>
      </section>

      <section className="entryDetailCard" aria-label={t('mobile.entryDetail.facts')}>
        {rows.map((row) => (
          <div className="entryDetailRow" key={row.label}>
            <span>{row.label}</span>
            <strong>{row.value}</strong>
          </div>
        ))}
      </section>

      <section className="entryDetailCard" aria-label={t('mobile.entryDetail.source')}>
        <div className="entryDetailRow">
          <span>{t('mobile.entryDetail.source')}</span>
          <strong>{entry.tags?.length ? entry.tags.join(', ') : t('mobile.entryDetail.manualSource')}</strong>
        </div>
      </section>

      <section className="entryDetailNote" aria-label={t('mobile.entryDetail.note')}>
        {entry.note || t('mobile.entryDetail.noNote')}
      </section>

      <ul className="entryDetailEditorList">
        <EntryDetailEditor
          account={account}
          accounts={accounts}
          categories={categories}
          entry={entry}
          isBusy={isSaving}
          openSignal={editorOpenSignal}
          onDeleteEntry={onDeleteEntry}
          onUpdateEntry={onUpdateEntry}
        />
      </ul>

      <footer className="entryDetailAudit">
        {auditRows.map((row) => (
          <p key={row.label}>
            <span>{row.label}</span>
            <strong>{row.value}</strong>
          </p>
        ))}
        <small>{t('mobile.entryDetail.editPolicy')}</small>
      </footer>
    </section>
  );
}

// entryTitle receives an entry and category context, then returns the display title.
function entryTitle(
  entry: Entry,
  category: Category | undefined,
  parentCategory: Category | undefined,
  fallback: string,
): string {
  return entry.note || entry.merchant || categoryPath(category, parentCategory) || fallback;
}

// categoryPath receives an optional category and parent and returns their readable hierarchy.
function categoryPath(category: Category | undefined, parentCategory: Category | undefined): string {
  return [parentCategory?.name, category?.name].filter(Boolean).join(' / ');
}

// memberName receives a user id and members, then returns the display name used in detail rows.
function memberName(userId: string, members: BookMember[]): string {
  const member = members.find((item) => item.userId === userId);
  return member?.displayName || userId;
}

// signedEntryAmount receives an entry and returns the signed amount shown on detail pages.
function signedEntryAmount(entry: Entry): number {
  if (
    entry.type === 'income' ||
    entry.type === 'refund' ||
    entry.type === 'reimbursement' ||
    entry.type === 'borrow' ||
    entry.type === 'repayment'
  ) {
    return Math.abs(entry.amountCents);
  }
  if (entry.type === 'transfer') {
    return entry.amountCents;
  }

  return -Math.abs(entry.amountCents);
}

// formatDetailDate receives an ISO timestamp and returns a stable UTC detail timestamp.
function formatDetailDate(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  const year = date.getUTCFullYear();
  const month = String(date.getUTCMonth() + 1).padStart(2, '0');
  const day = String(date.getUTCDate()).padStart(2, '0');
  const hour = String(date.getUTCHours()).padStart(2, '0');
  const minute = String(date.getUTCMinutes()).padStart(2, '0');
  return `${year}/${month}/${day} ${hour}:${minute}`;
}
