import { ArrowDownLeft, ArrowUpRight, ChevronDown } from 'lucide-react';
import {
  type KeyboardEvent as ReactKeyboardEvent,
  type PointerEvent as ReactPointerEvent,
  useId,
  useMemo,
  useRef,
  useState,
} from 'react';
import { useTranslation } from 'react-i18next';
import { type Account, type BookMember, type Category, type Entry } from '@/lib/api/ledger';
import { formatMoney } from '@/lib/money';
import { accountEntries } from './account-transaction-utils';
import './account-transactions.css';

type AccountTransactionsViewProps = {
  account?: Account;
  categories: Category[];
  entries: Entry[];
  error?: string;
  isLoading: boolean;
  members: BookMember[];
  onOpenEntry?: (entryId: string) => void;
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
  error,
  isLoading,
  members,
  onOpenEntry,
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
    <section
      className="tabPanel accountTransactionsPanel"
      aria-label={t('mobile.accountDetail.label', { name: account.name })}
    >
      <section className="accountBalanceHero" aria-label={t('mobile.accountDetail.balanceChart')}>
        <div className="accountBalanceHeroText">
          <span>{t('mobile.accountDetail.balance')}</span>
          <strong>{formatMoney(currentBalanceCents, account.currency)}</strong>
        </div>
        <InteractiveBalanceChart
          currency={account.currency}
          openingBalanceCents={account.openingBalanceCents}
          points={accountPoints}
        />
        <footer>
          <span>{t('mobile.accountDetail.totalIn', { amount: formatMoney(totalIncomeCents, account.currency) })}</span>
          <span>
            {t('mobile.accountDetail.totalOut', { amount: formatMoney(totalExpenseCents, account.currency) })}
          </span>
        </footer>
      </section>

      {isLoading ? <p className="mobileSearchMessage">{t('mobile.accountDetail.loading')}</p> : null}
      {error ? <p className="mobileInlineError">{error}</p> : null}

      {!isLoading && !error && monthGroups.length ? (
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
                      <button
                        type="button"
                        className="accountTransactionButton"
                        aria-label={t('mobile.transactions.openEntry', {
                          title: entryTitle(point.entry, categories, t('mobile.transactions.accountFallback')),
                        })}
                        onClick={() => onOpenEntry?.(point.entry.id)}
                      >
                        <span
                          className={`accountTransactionIcon ${point.deltaCents >= 0 ? 'accountTransactionIconIncome' : ''}`}
                        >
                          {point.deltaCents >= 0 ? <ArrowDownLeft size={20} /> : <ArrowUpRight size={20} />}
                        </span>
                        <div>
                          <strong>
                            {entryTitle(point.entry, categories, t('mobile.transactions.accountFallback'))}
                          </strong>
                          <small>
                            {t('mobile.accountDetail.entryMeta', {
                              member: memberName(point.entry.creatorUserId, members),
                              time: formatEntryTime(point.entry.occurredAt),
                            })}
                          </small>
                        </div>
                        <b className={point.deltaCents >= 0 ? 'accountTransactionIncome' : undefined}>
                          {formatSignedMoney(point.deltaCents, account.currency)}
                          <small>
                            {t('mobile.accountDetail.balanceAfter', {
                              amount: formatMoney(point.balanceCents, account.currency),
                            })}
                          </small>
                        </b>
                      </button>
                    </li>
                  ))}
                </ul>
              ) : null}
            </article>
          ))}
        </div>
      ) : null}

      {!isLoading && !error && !monthGroups.length ? (
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
    const id = Number.isNaN(date.getTime())
      ? 'unknown'
      : `${date.getUTCFullYear()}-${String(date.getUTCMonth() + 1).padStart(2, '0')}`;
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

type ChartPoint = {
  cents: number;
  label: string;
  deltaCents: number | null;
};

// Vertical inset (in viewBox units) that keeps the line and markers off the chart edges.
const CHART_TOP = 14;
const CHART_BOTTOM = 86;

// InteractiveBalanceChart receives balance points and renders a scrubbable account balance timeline.
function InteractiveBalanceChart({
  currency,
  openingBalanceCents,
  points,
}: {
  currency: string;
  openingBalanceCents: number;
  points: BalancePoint[];
}) {
  const { t } = useTranslation();
  const gradientId = useId();
  const trackRef = useRef<HTMLDivElement>(null);
  const series = useMemo(
    () => buildChartSeries(openingBalanceCents, points, t('mobile.accountDetail.opening')),
    [openingBalanceCents, points, t],
  );
  const pointCount = series.length;
  const [activeIndex, setActiveIndex] = useState(pointCount - 1);
  const [isScrubbing, setIsScrubbing] = useState(false);
  const active = Math.min(Math.max(activeIndex, 0), pointCount - 1);
  const activePoint = series[active] ?? series[pointCount - 1]!;

  const values = series.map((point) => point.cents);
  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = max - min;
  const xFor = (index: number) => (pointCount <= 1 ? 50 : (index / (pointCount - 1)) * 100);
  const yFor = (value: number) =>
    span === 0 ? 50 : CHART_TOP + ((max - value) / span) * (CHART_BOTTOM - CHART_TOP);
  const linePath =
    pointCount >= 2
      ? series
          .map((point, index) => `${index === 0 ? 'M' : 'L'} ${xFor(index).toFixed(2)} ${yFor(point.cents).toFixed(2)}`)
          .join(' ')
      : '';
  const areaPath = linePath ? `${linePath} L ${xFor(pointCount - 1).toFixed(2)} 100 L ${xFor(0).toFixed(2)} 100 Z` : '';
  const activeX = xFor(active);
  const tooltipLeft = Math.min(84, Math.max(16, activeX));
  const valueText = t('mobile.accountDetail.chartPoint', {
    date: activePoint.label,
    amount: formatMoney(activePoint.cents, currency),
  });

  // indexFromClientX receives a pointer x coordinate and returns the nearest data point index.
  function indexFromClientX(clientX: number): number {
    const rect = trackRef.current?.getBoundingClientRect();
    if (!rect || rect.width === 0 || pointCount <= 1) {
      return pointCount - 1;
    }

    const ratio = (clientX - rect.left) / rect.width;
    return Math.round(Math.min(1, Math.max(0, ratio)) * (pointCount - 1));
  }

  // handlePointerDown starts scrubbing and captures the pointer so drags continue outside the track.
  function handlePointerDown(event: ReactPointerEvent<HTMLDivElement>) {
    event.currentTarget.setPointerCapture?.(event.pointerId);
    setIsScrubbing(true);
    setActiveIndex(indexFromClientX(event.clientX));
  }

  // handlePointerMove tracks the active point under the pointer while hovering or dragging.
  function handlePointerMove(event: ReactPointerEvent<HTMLDivElement>) {
    setIsScrubbing(true);
    setActiveIndex(indexFromClientX(event.clientX));
  }

  // handlePointerLeave hides the scrubber for mouse users and resets to the latest balance.
  function handlePointerLeave(event: ReactPointerEvent<HTMLDivElement>) {
    if (event.pointerType !== 'mouse') {
      return;
    }

    setIsScrubbing(false);
    setActiveIndex(pointCount - 1);
  }

  // handleKeyDown moves the active point with arrow, Home, and End keys.
  function handleKeyDown(event: ReactKeyboardEvent<HTMLDivElement>) {
    let next: number;
    switch (event.key) {
      case 'ArrowRight':
      case 'ArrowUp':
        next = Math.min(pointCount - 1, active + 1);
        break;
      case 'ArrowLeft':
      case 'ArrowDown':
        next = Math.max(0, active - 1);
        break;
      case 'Home':
        next = 0;
        break;
      case 'End':
        next = pointCount - 1;
        break;
      default:
        return;
    }

    event.preventDefault();
    setIsScrubbing(true);
    setActiveIndex(next);
  }

  return (
    <div className="accountBalanceChart">
      <svg className="accountBalanceChartSvg" viewBox="0 0 100 100" preserveAspectRatio="none" aria-hidden="true">
        <defs>
          <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="oklch(100% 0 0 / 42%)" />
            <stop offset="100%" stopColor="oklch(100% 0 0 / 0%)" />
          </linearGradient>
        </defs>
        {areaPath ? <path className="accountBalanceArea" d={areaPath} fill={`url(#${gradientId})`} /> : null}
        {linePath ? <path className="accountBalanceLine" d={linePath} vectorEffect="non-scaling-stroke" /> : null}
      </svg>

      {isScrubbing ? (
        <span className="accountBalanceCrosshair" style={{ left: `${activeX}%` }} aria-hidden="true" />
      ) : null}
      <span
        className="accountBalanceDot"
        style={{ left: `${activeX}%`, top: `${yFor(activePoint.cents)}%` }}
        aria-hidden="true"
      />
      {isScrubbing ? (
        <div className="accountBalanceTooltip" style={{ left: `${tooltipLeft}%` }} aria-hidden="true">
          <span className="accountBalanceTooltipDate">{activePoint.label}</span>
          <strong>{formatMoney(activePoint.cents, currency)}</strong>
          {activePoint.deltaCents != null && activePoint.deltaCents !== 0 ? (
            <em className={activePoint.deltaCents > 0 ? 'accountBalanceTooltipUp' : undefined}>
              {formatSignedMoney(activePoint.deltaCents, currency)}
            </em>
          ) : null}
        </div>
      ) : null}

      <div
        ref={trackRef}
        className="accountBalanceChartTrack"
        role="slider"
        tabIndex={0}
        aria-label={t('mobile.accountDetail.balanceChart')}
        aria-orientation="horizontal"
        aria-valuemin={0}
        aria-valuemax={pointCount - 1}
        aria-valuenow={active}
        aria-valuetext={valueText}
        onKeyDown={handleKeyDown}
        onFocus={() => setIsScrubbing(true)}
        onBlur={() => {
          setIsScrubbing(false);
          setActiveIndex(pointCount - 1);
        }}
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerLeave={handlePointerLeave}
      />
    </div>
  );
}

// buildChartSeries receives opening balance and running points, then returns chart points with labels and deltas.
function buildChartSeries(openingBalanceCents: number, points: BalancePoint[], openingLabel: string): ChartPoint[] {
  return [
    { cents: openingBalanceCents, label: openingLabel, deltaCents: null },
    ...points.map((point) => ({
      cents: point.balanceCents,
      label: formatPointDate(point.entry.occurredAt),
      deltaCents: point.deltaCents,
    })),
  ];
}

// formatPointDate receives an ISO timestamp and returns a compact UTC calendar date for chart labels.
function formatPointDate(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return new Intl.DateTimeFormat('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    timeZone: 'UTC',
  }).format(date);
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
