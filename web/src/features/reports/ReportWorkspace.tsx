import type { TFunction } from 'i18next';
import { ChevronDown, ChevronLeft, ChevronRight, CircleDollarSign, Filter, PieChart } from 'lucide-react';
import type { KeyboardEvent } from 'react';
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  fetchAccounts,
  fetchAllEntries,
  fetchBooks,
  fetchCategories,
  fetchExchangeRates,
  type Account,
  type BookListItem,
  type Category,
  type Entry,
  type ExchangeRate,
} from '../../lib/api/ledger';
import { buildRateIndex, compactMoney, convertEntryAmountCents, formatMoney } from '../../lib/money';
import './reports.css';

type FlowFilter = 'all' | 'expense' | 'income';
type TimeMode = 'year' | 'month';
type ReportTab = 'trend' | 'category' | 'subcategory' | 'member' | 'account' | 'merchant' | 'tag';

type ReportData = {
  books: BookListItem[];
  accounts: Account[];
  categories: Category[];
  entries: Entry[];
  rates: ExchangeRate[];
};

type ReportWorkspaceProps = {
  refreshKey?: number;
};

type RankedItem = {
  id: string;
  label: string;
  amountCents: number;
  count: number;
  percent: number;
};

type DonutSegment = RankedItem & {
  color: string;
  dashArray: string;
  dashOffset: number;
};

const reportTabs: Array<{ id: ReportTab }> = [
  { id: 'trend' },
  { id: 'category' },
  { id: 'subcategory' },
  { id: 'member' },
  { id: 'account' },
  { id: 'merchant' },
  { id: 'tag' },
];

const reportColors = ['#2ec4b6', '#2eb6e6', '#5a7cff', '#ffb14a', '#8dd85f', '#ff7f66', '#a56dff', '#4db39f'];

const percent = new Intl.NumberFormat('en-US', {
  maximumFractionDigits: 2,
  minimumFractionDigits: 2,
});

// ReportWorkspace renders interactive reporting over real ledger entries.
export function ReportWorkspace({ refreshKey = 0 }: ReportWorkspaceProps) {
  const { t } = useTranslation();
  const [data, setData] = useState<ReportData>({ books: [], accounts: [], categories: [], entries: [], rates: [] });
  const [selectedBookId, setSelectedBookId] = useState('');
  const [flowFilter, setFlowFilter] = useState<FlowFilter>('expense');
  const [timeMode, setTimeMode] = useState<TimeMode>('month');
  const [cursorDate, setCursorDate] = useState(() => startOfMonth(new Date()));
  const [activeTab, setActiveTab] = useState<ReportTab>('category');
  const [isExpanded, setIsExpanded] = useState(false);
  const [isFilterOpen, setIsFilterOpen] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState('');

  const selectedBook = data.books.find((book) => book.id === selectedBookId) ?? data.books[0];
  const reportCurrency = selectedBook?.reportingCurrency ?? 'USD';
  const filteredEntries = useMemo(
    () => filterEntries(data.entries, flowFilter, timeMode, cursorDate),
    [cursorDate, data.entries, flowFilter, timeMode],
  );
  const rateIndex = useMemo(() => buildRateIndex(data.rates), [data.rates]);
  const rankedItems = useMemo(
    () => buildRankedItems(t, filteredEntries, activeTab, data.categories, data.accounts, reportCurrency, rateIndex),
    [activeTab, data.accounts, data.categories, filteredEntries, rateIndex, reportCurrency, t],
  );
  const visibleItems = isExpanded ? rankedItems : rankedItems.slice(0, 5);
  const totalCents = rankedItems.reduce((sum, item) => sum + item.amountCents, 0);
  const entryCount = filteredEntries.length;
  const conversionIssueCount = filteredEntries.filter((entry) => convertEntryAmountCents(entry, reportCurrency, rateIndex) === null).length;
  const segments = useMemo(() => buildDonutSegments(rankedItems), [rankedItems]);
  const trendRows = useMemo(
    () => buildTrendRows(data.entries, flowFilter, timeMode, cursorDate, reportCurrency, rateIndex),
    [cursorDate, data.entries, flowFilter, rateIndex, reportCurrency, timeMode],
  );
  const activeTabPanelId = `report-panel-${activeTab}`;
  const activeTabButtonId = `report-tab-${activeTab}`;

  useEffect(() => {
    let isActive = true;
    setIsLoading(true);
    setError('');
    Promise.all([fetchBooks(), fetchAccounts(), fetchExchangeRates()])
      .then(([books, accounts, rates]) => {
        if (!isActive) {
          return;
        }
        setData((current) => ({ ...current, books, accounts, rates }));
        setSelectedBookId((current) => current || books[0]?.id || '');
      })
      .catch(() => {
        if (isActive) {
          setError(t('reports.error.loadFailed'));
        }
      })
      .finally(() => {
        if (isActive) {
          setIsLoading(false);
        }
      });

    return () => {
      isActive = false;
    };
  }, [refreshKey, t]);

  useEffect(() => {
    if (!selectedBookId) {
      setData((current) => ({ ...current, categories: [], entries: [] }));
      return;
    }

    let isActive = true;
    setIsLoading(true);
    setError('');
    Promise.all([fetchCategories(selectedBookId), fetchAllEntries(selectedBookId)])
      .then(([categories, entries]) => {
        if (isActive) {
          setData((current) => ({ ...current, categories, entries }));
        }
      })
      .catch(() => {
        if (isActive) {
          setError(t('reports.error.dataFailed'));
        }
      })
      .finally(() => {
        if (isActive) {
          setIsLoading(false);
        }
      });

    return () => {
      isActive = false;
    };
  }, [refreshKey, selectedBookId, t]);

  // handleTimeStep receives a direction and moves the report cursor by month or year.
  function handleTimeStep(direction: -1 | 1) {
    setCursorDate((current) => {
      const next = new Date(current);
      if (timeMode === 'year') {
        next.setUTCFullYear(next.getUTCFullYear() + direction);
        return startOfYear(next);
      }
      next.setUTCMonth(next.getUTCMonth() + direction);
      return startOfMonth(next);
    });
  }

  // handleModeChange receives a time mode and normalizes the report cursor to that period.
  function handleModeChange(nextMode: TimeMode) {
    setTimeMode(nextMode);
    setCursorDate((current) => (nextMode === 'year' ? startOfYear(current) : startOfMonth(current)));
  }

  // handleTabKeyDown receives a keyboard event and moves focus through report tabs.
  function handleTabKeyDown(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) {
      return;
    }

    event.preventDefault();
    let nextIndex: number;
    if (event.key === 'Home') {
      nextIndex = 0;
    } else if (event.key === 'End') {
      nextIndex = reportTabs.length - 1;
    } else {
      const direction = event.key === 'ArrowRight' ? 1 : -1;
      nextIndex = (index + direction + reportTabs.length) % reportTabs.length;
    }

    const nextTab = reportTabs[nextIndex];
    setActiveTab(nextTab.id);
    setIsExpanded(false);
    window.requestAnimationFrame(() => document.getElementById(`report-tab-${nextTab.id}`)?.focus());
  }

  return (
    <section className="reportWorkspace" aria-label={t('reports.title')}>
      <div className="reportTopbar">
        <div>
          <p className="eyebrow">{t('reports.eyebrow')}</p>
          <h2>{t('reports.heading')}</h2>
        </div>
        <div className="reportActions">
          <select
            aria-label={t('reports.a11y.book')}
            value={selectedBook?.id ?? ''}
            onChange={(event) => {
              setSelectedBookId(event.target.value);
              setIsExpanded(false);
            }}
            disabled={!data.books.length}
          >
            {data.books.length ? data.books.map((book) => <option key={book.id} value={book.id}>{book.name}</option>) : <option>{t('common.noBookYet')}</option>}
          </select>
          <button
            className="iconButton"
            type="button"
            aria-expanded={isFilterOpen}
            aria-controls="report-filter-panel"
            aria-label={t('reports.a11y.filters')}
            onClick={() => setIsFilterOpen((current) => !current)}
          >
            <Filter size={18} />
          </button>
        </div>
      </div>

      {isFilterOpen ? (
        <div className="reportFilterPanel" id="report-filter-panel" role="region" aria-label={t('reports.a11y.activeFilters')}>
          <span>
            <strong>{t('reports.filter.book')}</strong>
            {selectedBook?.name ?? t('reports.noBook')}
          </span>
          <span>
            <strong>{t('reports.filter.flow')}</strong>
            {t(`common.flow.${flowFilter}`)}
          </span>
          <span>
            <strong>{t('reports.filter.period')}</strong>
            {formatPeriod(cursorDate, timeMode)}
          </span>
          <span>
            <strong>{t('reports.filter.total')}</strong>
            {formatMoney(totalCents, reportCurrency)}
          </span>
        </div>
      ) : null}

      <div className="reportControls" aria-label={t('reports.a11y.controls')}>
        <div className="reportSegmented" aria-label={t('reports.a11y.flowFilter')}>
          {(['all', 'expense', 'income'] as FlowFilter[]).map((filter) => (
            <button
              key={filter}
              type="button"
              className={flowFilter === filter ? 'reportSegmentActive' : ''}
              onClick={() => {
                setFlowFilter(filter);
                setIsExpanded(false);
              }}
            >
              {t(`common.flow.${filter}`)}
            </button>
          ))}
        </div>
        <div className="reportSegmented" aria-label={t('reports.a11y.timeMode')}>
          {(['year', 'month'] as TimeMode[]).map((mode) => (
            <button
              key={mode}
              type="button"
              className={timeMode === mode ? 'reportSegmentActive' : ''}
              onClick={() => handleModeChange(mode)}
            >
              {t(`reports.timeMode.${mode}`)}
            </button>
          ))}
        </div>
        <div className="periodStepper" aria-label={t('reports.a11y.period')}>
          <button type="button" aria-label={t('reports.a11y.prevPeriod')} onClick={() => handleTimeStep(-1)}>
            <ChevronLeft size={18} />
          </button>
          <strong>{formatPeriod(cursorDate, timeMode)}</strong>
          <button type="button" aria-label={t('reports.a11y.nextPeriod')} onClick={() => handleTimeStep(1)}>
            <ChevronRight size={18} />
          </button>
        </div>
      </div>

      <div className="reportTabs" role="tablist" aria-label={t('reports.a11y.dimensions')}>
        {reportTabs.map((tab, index) => (
          <button
            key={tab.id}
            id={`report-tab-${tab.id}`}
            type="button"
            role="tab"
            aria-controls={`report-panel-${tab.id}`}
            aria-selected={activeTab === tab.id}
            className={activeTab === tab.id ? 'reportTabActive' : ''}
            tabIndex={activeTab === tab.id ? 0 : -1}
            onKeyDown={(event) => handleTabKeyDown(event, index)}
            onClick={() => {
              setActiveTab(tab.id);
              setIsExpanded(false);
            }}
          >
            {t(`reports.tabs.${tab.id}`)}
          </button>
        ))}
      </div>

      {error ? <p className="authError">{error}</p> : null}
      {isLoading ? <p className="authStatus">{t('reports.loading')}</p> : null}
      {conversionIssueCount > 0 ? (
        <p className="authStatus">{t('reports.conversionIssue', { value: conversionIssueCount })}</p>
      ) : null}

      {activeTab === 'trend' ? (
        <TrendPanel
          currencyCode={reportCurrency}
          entryCount={entryCount}
          labelledBy={activeTabButtonId}
          panelId={activeTabPanelId}
          rows={trendRows}
          totalCents={totalCents}
        />
      ) : (
        <section className="reportPanel" id={activeTabPanelId} role="tabpanel" aria-labelledby={activeTabButtonId}>
          <div className="reportPanelHeader">
            <div>
              <p className="eyebrow">{reportTitle(t, activeTab, flowFilter)}</p>
              <h3>{reportHeading(t, activeTab, flowFilter)}</h3>
            </div>
            <span>{t('common.entriesCount', { value: entryCount })}</span>
          </div>

          <div className="reportBody">
            <DonutChart currencyCode={reportCurrency} segments={segments} totalCents={totalCents} label={reportHeading(t, activeTab, flowFilter)} />
            <RankedList currencyCode={reportCurrency} items={visibleItems} totalCents={totalCents} />
          </div>

          {rankedItems.length > 5 ? (
            <button className="expandReport" type="button" onClick={() => setIsExpanded((current) => !current)}>
              <span>{isExpanded ? t('reports.showLess') : t('reports.showAll')}</span>
              <ChevronDown size={16} className={isExpanded ? 'expandIconRotated' : ''} />
            </button>
          ) : null}
        </section>
      )}
    </section>
  );
}

// DonutChart receives report segments and renders an accessible proportional donut chart.
function DonutChart({
  segments,
  totalCents,
  label,
  currencyCode,
}: {
  segments: DonutSegment[];
  totalCents: number;
  label: string;
  currencyCode: string;
}) {
  const { t } = useTranslation();
  const radius = 70;
  const stroke = 32;
  const center = 94;
  const hasData = totalCents > 0;

  return (
    <figure className="donutFigure">
      <svg viewBox="0 0 188 188" role="img" aria-labelledby="reportDonutTitle reportDonutDescription">
        <title id="reportDonutTitle">{label}</title>
        <desc id="reportDonutDescription">
          {hasData
            ? t('reports.donut.desc', { value: segments.length, amount: formatMoney(totalCents, currencyCode) })
            : t('reports.donut.empty')}
        </desc>
        <circle cx={center} cy={center} r={radius} fill="none" stroke="oklch(91% 0.018 112)" strokeWidth={stroke} />
        {segments.map((segment) => (
          <circle
            key={segment.id}
            cx={center}
            cy={center}
            r={radius}
            fill="none"
            stroke={segment.color}
            strokeDasharray={segment.dashArray}
            strokeDashoffset={segment.dashOffset}
            strokeLinecap="butt"
            strokeWidth={stroke}
          />
        ))}
        <text x="94" y="84" textAnchor="middle">
          {t('common.total')}
        </text>
        <text className="donutAmount" x="94" y="112" textAnchor="middle">
          {compactMoney(totalCents)}
        </text>
      </svg>
      {hasData ? (
        <figcaption>
          {segments.slice(0, 4).map((segment) => (
            <span key={segment.id}>
              <i style={{ background: segment.color }} />
              {segment.label} {percent.format(segment.percent)}%
            </span>
          ))}
        </figcaption>
      ) : (
        <figcaption>{t('reports.donut.emptyCaption')}</figcaption>
      )}
    </figure>
  );
}

// RankedList receives ranked report rows and returns a ranked amount list.
function RankedList({ items, totalCents, currencyCode }: { items: RankedItem[]; totalCents: number; currencyCode: string }) {
  const { t } = useTranslation();
  if (!items.length) {
    return (
      <div className="reportEmpty">
        <CircleDollarSign size={26} />
        <p>{t('reports.rankedEmpty')}</p>
      </div>
    );
  }

  return (
    <ol className="rankedList">
      {items.map((item, index) => (
        <li key={item.id}>
          <span className="rankIcon" style={{ background: reportColors[index % reportColors.length] }}>
            {index + 1}
          </span>
          <div className="rankMain">
            <div>
              <strong>{item.label}</strong>
              <b>{formatMoney(item.amountCents, currencyCode)}</b>
            </div>
            <div className="rankBar" aria-hidden="true">
              <span style={{ width: `${totalCents > 0 ? Math.max(3, item.percent) : 0}%`, background: reportColors[index % reportColors.length] }} />
            </div>
            <small>
              {t('common.entriesCount', { value: item.count })} <em>{percent.format(item.percent)}%</em>
            </small>
          </div>
        </li>
      ))}
    </ol>
  );
}

// TrendPanel receives period buckets and renders income, expense, and net movement.
function TrendPanel({
  rows,
  totalCents,
  entryCount,
  panelId,
  labelledBy,
  currencyCode,
}: {
  rows: RankedItem[];
  totalCents: number;
  entryCount: number;
  panelId: string;
  labelledBy: string;
  currencyCode: string;
}) {
  const { t } = useTranslation();
  const max = Math.max(...rows.map((row) => row.amountCents), 1);

  return (
    <section className="reportPanel trendPanel" id={panelId} role="tabpanel" aria-labelledby={labelledBy}>
      <div className="reportPanelHeader">
        <div>
          <p className="eyebrow">{t('reports.tabs.trend')}</p>
          <h3>{t('reports.trend.heading')}</h3>
        </div>
        <span>{t('common.entriesCount', { value: entryCount })}</span>
      </div>
      <div className="trendSummary">
        <PieChart size={24} />
        <strong>{formatMoney(totalCents, currencyCode)}</strong>
        <span>{t('reports.trend.filteredTotal')}</span>
      </div>
      <ol className="trendBars">
        {rows.map((row) => (
          <li key={row.id}>
            <span>{row.label}</span>
            <div className="trendTrack">
              <i style={{ width: `${Math.max(3, (row.amountCents / max) * 100)}%` }} />
            </div>
            <strong>{formatMoney(row.amountCents, currencyCode)}</strong>
          </li>
        ))}
      </ol>
    </section>
  );
}

// filterEntries receives entries and report controls and returns matching rows.
function filterEntries(entries: Entry[], flowFilter: FlowFilter, timeMode: TimeMode, cursorDate: Date): Entry[] {
  const [start, end] = periodBounds(cursorDate, timeMode);
  return entries.filter((entry) => {
    const occurredAt = new Date(entry.occurredAt);
    if (Number.isNaN(occurredAt.getTime()) || occurredAt < start || occurredAt >= end) {
      return false;
    }
    if (flowFilter === 'all') {
      return entry.type === 'expense' || entry.type === 'income';
    }

    return entry.type === flowFilter;
  });
}

// buildRankedItems receives the translator, entries, a dimension, and currency and returns sorted base-currency report rows.
function buildRankedItems(
  t: TFunction,
  entries: Entry[],
  tab: ReportTab,
  categories: Category[],
  accounts: Account[],
  currencyCode: string,
  rates: Map<string, number>,
): RankedItem[] {
  const totals = new Map<string, RankedItem>();
  for (const entry of entries) {
    const amountCents = convertEntryAmountCents(entry, currencyCode, rates);
    if (amountCents === null) {
      continue;
    }
    for (const key of dimensionKeys(t, entry, tab, categories, accounts)) {
      const current = totals.get(key.id) ?? { ...key, amountCents: 0, count: 0, percent: 0 };
      current.amountCents += amountCents;
      current.count += 1;
      totals.set(key.id, current);
    }
  }

  const total = Array.from(totals.values()).reduce((sum, item) => sum + item.amountCents, 0);
  return Array.from(totals.values())
    .map((item) => ({ ...item, percent: total > 0 ? (item.amountCents / total) * 100 : 0 }))
    .sort((first, second) => second.amountCents - first.amountCents);
}

// buildDonutSegments receives ranked items and returns SVG dash settings for a donut chart.
function buildDonutSegments(items: RankedItem[]): DonutSegment[] {
  const total = items.reduce((sum, item) => sum + item.amountCents, 0);
  const circumference = 2 * Math.PI * 70;
  let offset = 0;
  return items.slice(0, 8).map((item, index) => {
    const length = total > 0 ? (item.amountCents / total) * circumference : 0;
    const segment = {
      ...item,
      color: reportColors[index % reportColors.length],
      dashArray: `${length} ${circumference - length}`,
      dashOffset: -offset,
    };
    offset += length;
    return segment;
  });
}

// buildTrendRows receives entries and controls and returns month or day trend buckets in base currency.
function buildTrendRows(
  entries: Entry[],
  flowFilter: FlowFilter,
  timeMode: TimeMode,
  cursorDate: Date,
  currencyCode: string,
  rates: Map<string, number>,
): RankedItem[] {
  const rows = timeMode === 'year' ? yearBuckets(cursorDate) : monthBuckets(cursorDate);
  const index = new Map(rows.map((row) => [row.id, row]));
  for (const entry of filterEntries(entries, flowFilter, timeMode, cursorDate)) {
    const amountCents = convertEntryAmountCents(entry, currencyCode, rates);
    if (amountCents === null) {
      continue;
    }
    const occurredAt = new Date(entry.occurredAt);
    const key = timeMode === 'year' ? `${occurredAt.getUTCFullYear()}-${String(occurredAt.getUTCMonth() + 1).padStart(2, '0')}` : occurredAt.toISOString().slice(0, 10);
    const current = index.get(key);
    if (!current) {
      continue;
    }
    current.amountCents += amountCents;
    current.count += 1;
  }

  const total = rows.reduce((sum, row) => sum + row.amountCents, 0);
  return rows.map((row) => ({ ...row, percent: total > 0 ? (row.amountCents / total) * 100 : 0 }));
}

// dimensionKeys receives the translator, an entry, and the active tab and returns grouping keys.
function dimensionKeys(t: TFunction, entry: Entry, tab: ReportTab, categories: Category[], accounts: Account[]): Array<Pick<RankedItem, 'id' | 'label'>> {
  const category = categories.find((item) => item.id === entry.categoryId);
  const account = accounts.find((item) => item.id === entry.accountId);
  switch (tab) {
    case 'category':
    case 'subcategory':
      return [{ id: category?.id ?? 'uncategorized', label: category?.name ?? t('reports.labels.uncategorized') }];
    case 'member': {
      const member = (entry as Entry & { member?: string }).member;
      return [{ id: member || 'no-member', label: member || t('reports.labels.noMember') }];
    }
    case 'account':
      return [{ id: account?.id ?? 'unassigned-account', label: account?.name ?? t('reports.labels.unassignedAccount') }];
    case 'merchant':
      return [{ id: entry.merchant || 'no-merchant', label: entry.merchant || t('reports.labels.noMerchant') }];
    case 'tag':
      return entry.tags?.length ? entry.tags.map((tag) => ({ id: tag, label: tag })) : [{ id: 'no-tag', label: t('reports.labels.noTag') }];
    case 'trend':
      return [{ id: 'trend', label: t('reports.tabs.trend') }];
    default:
      return [{ id: 'other', label: t('reports.labels.other') }];
  }
}

// periodBounds receives a cursor date and mode and returns UTC inclusive-exclusive bounds.
function periodBounds(cursorDate: Date, timeMode: TimeMode): [Date, Date] {
  if (timeMode === 'year') {
    const start = startOfYear(cursorDate);
    const end = new Date(start);
    end.setUTCFullYear(end.getUTCFullYear() + 1);
    return [start, end];
  }

  const start = startOfMonth(cursorDate);
  const end = new Date(start);
  end.setUTCMonth(end.getUTCMonth() + 1);
  return [start, end];
}

// yearBuckets receives a cursor date and returns monthly report buckets for that year.
function yearBuckets(cursorDate: Date): RankedItem[] {
  const year = cursorDate.getUTCFullYear();
  return Array.from({ length: 12 }, (_, month) => ({
    id: `${year}-${String(month + 1).padStart(2, '0')}`,
    label: new Intl.DateTimeFormat('en-US', { month: 'short', timeZone: 'UTC' }).format(new Date(Date.UTC(year, month, 1))),
    amountCents: 0,
    count: 0,
    percent: 0,
  }));
}

// monthBuckets receives a cursor date and returns daily report buckets for that month.
function monthBuckets(cursorDate: Date): RankedItem[] {
  const start = startOfMonth(cursorDate);
  const end = new Date(start);
  end.setUTCMonth(end.getUTCMonth() + 1);
  const days = Math.round((end.getTime() - start.getTime()) / 86_400_000);
  return Array.from({ length: days }, (_, index) => {
    const day = new Date(start);
    day.setUTCDate(day.getUTCDate() + index);
    return {
      id: day.toISOString().slice(0, 10),
      label: String(day.getUTCDate()).padStart(2, '0'),
      amountCents: 0,
      count: 0,
      percent: 0,
    };
  });
}

// startOfYear receives a date and returns the first UTC instant of that year.
function startOfYear(date: Date): Date {
  return new Date(Date.UTC(date.getUTCFullYear(), 0, 1));
}

// startOfMonth receives a date and returns the first UTC instant of that month.
function startOfMonth(date: Date): Date {
  return new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), 1));
}

// formatPeriod receives a cursor date and mode and returns a compact period label.
function formatPeriod(date: Date, mode: TimeMode): string {
  if (mode === 'year') {
    return String(date.getUTCFullYear());
  }

  return `${date.getUTCFullYear()}/${String(date.getUTCMonth() + 1).padStart(2, '0')}`;
}

// reportTitle receives the translator and active controls and returns the compact report eyebrow.
function reportTitle(t: TFunction, tab: ReportTab, flowFilter: FlowFilter): string {
  return t('reports.panel.eyebrow', { dimension: t(`reports.tabs.${tab}`), flow: t(`common.flow.${flowFilter}`) });
}

// reportHeading receives the translator and active controls and returns a readable report heading.
function reportHeading(t: TFunction, tab: ReportTab, flowFilter: FlowFilter): string {
  const target = flowFilter === 'all' ? t('reports.cashflow') : t(`common.flow.${flowFilter}`);
  return t('reports.panel.heading', { dimension: t(`reports.tabs.${tab}`), target });
}
