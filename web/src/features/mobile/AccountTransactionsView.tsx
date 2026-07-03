import { ArrowDownLeft, ArrowUpRight, ChevronDown } from 'lucide-react';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { type Account, type BookMember, type Category, type Entry } from '../../lib/api/ledger';
import { formatMoney } from '../../lib/money';
import { accountEntries } from './account-transaction-utils';
import './account-transactions.css';

type AccountTransactionsViewProps = {
  account?: Account;
  categories: Category[];
  entries: Entry[];
  isLoading: boolean;
  members: BookMember[];
};

type BalancePoint = {
  entry: Entry;
  deltaCents: number;
  balanceCents: number;
};

type MonthGroup = {
  id: string;
  label: string;
  year: string;
  incomeCents: number;
  expenseCents: number;
  endingBalanceCents: number;
  expanded: boolean;
  points: BalancePoint[];
};

// AccountTransactionsView receives one account and returns its balance timeline plus monthly transactions.
export function AccountTransactionsView({
  account,
  categories,
  entries,
  isLoading,
  members,
}: AccountTransactionsViewProps) {
  const { t } = useTranslation();
  const [expandedMonthIds, setExpandedMonthIds] = useState<ReadonlySet<string>>(() => new Set());
  const accountPoints = useMemo(() => (account ? buildBalancePoints(account, entries) : []), [account, entries]);
  const monthGroups = useMemo(
    () => buildMonthGroups(accountPoints, expandedMonthIds),
    [accountPoints, expandedMonthIds],
  );
  const currentBalanceCents = accountPoints.at(-1)?.balanceCents ?? account?.openingBalanceCents ?? 0;
  const totalIncomeCents = accountPoints.reduce((sum, point) => sum + Math.max(0, point.deltaCents), 0);
  const totalExpenseCents = Math.abs(accountPoints.reduce((sum, point) => sum + Math.min(0, point.deltaCents), 0));

  // handleToggleMonth receives a month id and toggles transaction visibility for that month.
  function handleToggleMonth(monthId: string) {
    setExpandedMonthIds((current) => {
      const next = new Set(current);
      if (next.has(monthId)) {
        next.delete(monthId);
      } else {
        next.add(monthId);
      }

      return next;
    });
  }

  if (!account) {
    return (
      <section className="tabPanel accountTransactionsPanel" aria-label={t('mobile.accountDetail.title')}>
        <p className="emptyState">{t('mobile.accountDetail.notFound')}</p>
      </section>
    );
  }

  return (
    <section className="tabPanel accountTransactionsPanel" aria-label={t('mobile.accountDetail.label', { name: account.name })}>
      <section className="accountBalanceHero" aria-label={t('mobile.accountDetail.balanceChart')}>
        <div className="accountBalanceHeroText">
          <span>{t('mobile.accountDetail.balance')}</span>
          <strong>{formatMoney(currentBalanceCents, account.currency)}</strong>
        </div>
        <BalanceSparkline points={accountPoints} openingBalanceCents={account.openingBalanceCents} />
        <footer>
          <span>{t('mobile.accountDetail.totalIn', { amount: formatMoney(totalIncomeCents, account.currency) })}</span>
          <span>{t('mobile.accountDetail.totalOut', { amount: formatMoney(totalExpenseCents, account.currency) })}</span>
        </footer>
      </section>

      {isLoading ? <p className="mobileSearchMessage">{t('mobile.accountDetail.loading')}</p> : null}

      {!isLoading && monthGroups.length ? (
        <div className="accountMonthList">
          {monthGroups.map((month) => (
            <article key={month.id} className={`accountMonth ${month.expanded ? 'accountMonthOpen' : ''}`}>
              <button
                type="button"
                className="accountMonthHeader"
                aria-expanded={month.expanded}
                onClick={() => handleToggleMonth(month.id)}
              >
                <span>
                  <strong>{month.label}</strong>
                  <small>{month.year}</small>
                </span>
                <em>
                  {t('mobile.accountDetail.monthFlows', {
                    income: formatMoney(month.incomeCents, account.currency),
                    expense: formatMoney(month.expenseCents, account.currency),
                  })}
                </em>
                <b>
                  {formatMoney(month.endingBalanceCents, account.currency)}
                  <small>{t('mobile.accountDetail.balance')}</small>
                </b>
                <ChevronDown size={18} aria-hidden="true" />
              </button>

              {month.expanded ? (
                <ul className="accountTransactionList">
                  {[...month.points].reverse().map((point) => (
                    <li key={point.entry.id}>
                      <span className={`accountTransactionIcon ${point.deltaCents >= 0 ? 'accountTransactionIconIncome' : ''}`}>
                        {point.deltaCents >= 0 ? <ArrowDownLeft size={20} /> : <ArrowUpRight size={20} />}
                      </span>
                      <div>
                        <strong>{entryTitle(point.entry, categories, t('mobile.transactions.accountFallback'))}</strong>
                        <small>
                          {t('mobile.accountDetail.entryMeta', {
                            member: memberName(point.entry.creatorUserId, members),
                            time: formatEntryTime(point.entry.occurredAt),
                          })}
                        </small>
                      </div>
                      <b className={point.deltaCents >= 0 ? 'accountTransactionIncome' : undefined}>
                        {formatSignedMoney(point.deltaCents, account.currency)}
                        <small>{t('mobile.accountDetail.balanceAfter', { amount: formatMoney(point.balanceCents, account.currency) })}</small>
                      </b>
                    </li>
                  ))}
                </ul>
              ) : null}
            </article>
          ))}
        </div>
      ) : null}

      {!isLoading && !monthGroups.length ? (
        <p className="emptyState">{t('mobile.accountDetail.empty')}</p>
      ) : null}
    </section>
  );
}

// buildBalancePoints receives an account and entries, then returns chronological running balances.
function buildBalancePoints(account: Account, entries: Entry[]): BalancePoint[] {
  let balanceCents = account.openingBalanceCents;
  return accountEntries(account.id, entries)
    .sort((left, right) => new Date(left.occurredAt).getTime() - new Date(right.occurredAt).getTime())
    .map((entry) => {
      const deltaCents = accountDeltaCents(account.id, entry);
      balanceCents += deltaCents;
      return { entry, deltaCents, balanceCents };
    });
}

// accountDeltaCents receives an account id and entry, then returns the account balance change.
function accountDeltaCents(accountId: string, entry: Entry): number {
  if (entry.destinationAccountId === accountId && entry.type === 'transfer') {
    return Math.abs(entry.amountCents);
  }
  if (entry.accountId !== accountId) {
    return 0;
  }
  if (['income', 'refund', 'reimbursement', 'borrow', 'repayment'].includes(entry.type)) {
    return Math.abs(entry.amountCents);
  }

  return -Math.abs(entry.amountCents);
}

// buildMonthGroups receives balance points and returns newest-first month summaries.
function buildMonthGroups(points: BalancePoint[], expandedMonthIds: ReadonlySet<string>): MonthGroup[] {
  const groups = new Map<string, BalancePoint[]>();
  for (const point of points) {
    const date = new Date(point.entry.occurredAt);
    const id = Number.isNaN(date.getTime()) ? 'unknown' : `${date.getUTCFullYear()}-${String(date.getUTCMonth() + 1).padStart(2, '0')}`;
    groups.set(id, [...(groups.get(id) ?? []), point]);
  }

  return [...groups.entries()]
    .sort(([left], [right]) => right.localeCompare(left))
    .map(([id, monthPoints], index) => {
      const lastPoint = monthPoints.at(-1);
      const date = new Date(monthPoints[0]?.entry.occurredAt ?? '');
      const fallback = id === 'unknown';
      return {
        id,
        label: fallback ? id : new Intl.DateTimeFormat('en-US', { month: 'long', timeZone: 'UTC' }).format(date),
        year: fallback ? '' : String(date.getUTCFullYear()),
        incomeCents: monthPoints.reduce((sum, point) => sum + Math.max(0, point.deltaCents), 0),
        expenseCents: Math.abs(monthPoints.reduce((sum, point) => sum + Math.min(0, point.deltaCents), 0)),
        endingBalanceCents: lastPoint?.balanceCents ?? 0,
        expanded: expandedMonthIds.has(id) || index === 0,
        points: monthPoints,
      };
    });
}

// BalanceSparkline receives balance points and renders a compact account balance chart.
function BalanceSparkline({ openingBalanceCents, points }: { openingBalanceCents: number; points: BalancePoint[] }) {
  const values = [openingBalanceCents, ...points.map((point) => point.balanceCents)];
  const min = Math.min(...values);
  const max = Math.max(...values);
  const range = Math.max(1, max - min);
  const coordinates = values.map((value, index) => {
    const x = values.length === 1 ? 0 : (index / (values.length - 1)) * 100;
    const y = 34 - ((value - min) / range) * 28;
    return `${x.toFixed(2)},${y.toFixed(2)}`;
  });

  return (
    <svg className="accountBalanceSparkline" viewBox="0 0 100 38" preserveAspectRatio="none" aria-hidden="true">
      <polyline points={coordinates.join(' ')} />
    </svg>
  );
}

// entryTitle receives an entry and categories, then returns the best transaction title.
function entryTitle(entry: Entry, categories: Category[], fallback: string): string {
  const category = categories.find((item) => item.id === entry.categoryId);
  const parent = categories.find((item) => item.id === category?.parentId);
  return entry.note || entry.merchant || [parent?.name, category?.name].filter(Boolean).join(' / ') || fallback;
}

// memberName receives a user id and members, then returns the display name used in transaction details.
function memberName(userId: string, members: BookMember[]): string {
  const member = members.find((item) => item.userId === userId);
  return member?.displayName || userId;
}

// formatEntryTime receives an ISO timestamp and returns compact UTC transaction time.
function formatEntryTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return new Intl.DateTimeFormat('en-US', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    timeZone: 'UTC',
  }).format(date);
}

// formatSignedMoney receives a signed cent amount and returns user-facing money text.
function formatSignedMoney(cents: number, currency: string): string {
  const prefix = cents > 0 ? '+' : cents < 0 ? '-' : '';
  return `${prefix}${formatMoney(Math.abs(cents), currency)}`;
}
