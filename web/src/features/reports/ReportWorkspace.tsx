import type { TFunction } from 'i18next';
import { ChevronLeft, ChevronRight, Filter } from 'lucide-react';
import type { KeyboardEvent } from 'react';
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate } from 'react-router';
import {
  fetchAccounts,
  fetchAllEntries,
  fetchBookMembers,
  fetchBooks,
  fetchCategories,
  fetchExchangeRates,
  type Account,
  type BookListItem,
  type BookMember,
  type Category,
  type Entry,
  type ExchangeRate,
} from '../../lib/api/ledger';
import { buildRateIndex, convertEntryAmountCents, formatMoney } from '../../lib/money';
import {
  ReportBreakdownPanel,
  TrendPanel,
  type DonutSegment,
  type RankedItem,
  type TrendBucket,
  type TrendData,
} from './ReportVisuals';
import { reportColors } from './reportColors';
import './reports.css';

type FlowFilter = 'all' | 'expense' | 'income';
type ReportFlow = 'expense' | 'income' | 'balance';
type TimeMode = 'year' | 'month';
type ReportTab = 'trend' | 'category' | 'subcategory' | 'member' | 'account' | 'merchant' | 'tag';
type SectionReportTab = Extract<ReportTab, 'category' | 'subcategory' | 'member'>;
type ReportFlowSection = {
  flow: ReportFlow;
  items: RankedItem[];
  segments: DonutSegment[];
};

type ReportData = {
  books: BookListItem[];
  members: BookMember[];
  accounts: Account[];
  categories: Category[];
  entries: Entry[];
  rates: ExchangeRate[];
};

type ReportWorkspaceProps = {
  baseCurrency?: string;
  refreshKey?: number;
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

// ReportWorkspace renders interactive reporting over real ledger entries.
export function ReportWorkspace({ baseCurrency, refreshKey = 0 }: ReportWorkspaceProps) {
  const { t } = useTranslation();
  const location = useLocation();
  const navigate = useNavigate();
  const [data, setData] = useState<ReportData>({ books: [], members: [], accounts: [], categories: [], entries: [], rates: [] });
  const [selectedBookId, setSelectedBookId] = useState('');
  const [flowFilter, setFlowFilter] = useState<FlowFilter>('all');
  const [timeMode, setTimeMode] = useState<TimeMode>('month');
  const [cursorDate, setCursorDate] = useState(() => startOfMonth(new Date()));
  const [isExpanded, setIsExpanded] = useState(false);
  const [isFilterOpen, setIsFilterOpen] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState('');

  const selectedBook = data.books.find((book) => book.id === selectedBookId) ?? data.books[0];
  const activeTab = reportTabFromPath(location.pathname) ?? 'category';
  const reportCurrency = baseCurrency ?? selectedBook?.reportingCurrency ?? 'USD';
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
  const categorySections = useMemo(
    () => buildDimensionFlowSections(t, data.entries, 'category', data.categories, data.accounts, timeMode, cursorDate, reportCurrency, rateIndex),
    [cursorDate, data.accounts, data.categories, data.entries, rateIndex, reportCurrency, t, timeMode],
  );
  const subcategorySections = useMemo(
    () => buildDimensionFlowSections(t, data.entries, 'subcategory', data.categories, data.accounts, timeMode, cursorDate, reportCurrency, rateIndex),
    [cursorDate, data.accounts, data.categories, data.entries, rateIndex, reportCurrency, t, timeMode],
  );
  const memberSections = useMemo(
    () => buildMemberFlowSections(t, data.entries, data.members, timeMode, cursorDate, reportCurrency, rateIndex),
    [cursorDate, data.entries, data.members, rateIndex, reportCurrency, t, timeMode],
  );
  const trendRows = useMemo(
    () => buildTrendData(data.entries, timeMode, cursorDate, reportCurrency, rateIndex),
    [cursorDate, data.entries, rateIndex, reportCurrency, timeMode],
  );
  const memberSingleFlow: Extract<FlowFilter, 'expense' | 'income'> = flowFilter === 'income' ? 'income' : 'expense';
  const memberSingleSection = memberSections.find((section) => section.flow === memberSingleFlow) ?? memberSections[0];
  const memberSingleItems = memberSingleSection?.items ?? [];
  const memberSingleSegments = memberSingleSection?.segments ?? [];
  const sectionReportTab: SectionReportTab = isSectionReportTab(activeTab) ? activeTab : 'category';
  const allFlowSections = sectionReportTab === 'category' ? categorySections : sectionReportTab === 'subcategory' ? subcategorySections : memberSections;
  const visibleTotalCents = activeTab === 'trend' || flowFilter === 'all' ? trendRows.balanceCents : totalCents;
  const shouldShowFlowSections = flowFilter === 'all' && isSectionReportTab(activeTab);
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
      setData((current) => ({ ...current, members: [], categories: [], entries: [] }));
      return;
    }

    let isActive = true;
    setIsLoading(true);
    setError('');
    Promise.all([fetchCategories(selectedBookId), fetchAllEntries(selectedBookId), fetchBookMembers(selectedBookId)])
      .then(([categories, entries, members]) => {
        if (isActive) {
          setData((current) => ({ ...current, members, categories, entries }));
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

  useEffect(() => {
    if (location.pathname === '/reports' || !reportTabFromPath(location.pathname)) {
      navigate(reportTabPath('category'), { replace: true });
    }
  }, [location.pathname, navigate]);

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
    navigate(reportTabPath(nextTab.id));
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
            {formatMoney(visibleTotalCents, reportCurrency)}
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
              navigate(reportTabPath(tab.id));
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
          labelledBy={activeTabButtonId}
          panelId={activeTabPanelId}
          trend={trendRows}
        />
      ) : shouldShowFlowSections ? (
        <section className="categorySplitPanel" id={activeTabPanelId} role="tabpanel" aria-labelledby={activeTabButtonId}>
          {allFlowSections.map((section) => {
            const isMemberSection = sectionReportTab === 'member';
            const sectionTotalCents = isMemberSection ? section.items[0]?.amountCents ?? 0 : section.items.reduce((sum, item) => sum + item.amountCents, 0);
            const sectionEntryCount = isMemberSection ? section.items[0]?.count ?? 0 : section.items.reduce((sum, item) => sum + item.count, 0);
            return (
              <ReportBreakdownPanel
                className="reportPanel categoryFlowPanel"
                currencyCode={reportCurrency}
                entryCount={sectionEntryCount}
                heading={flowSectionHeading(t, sectionReportTab, section.flow)}
                isExpanded={isExpanded}
                items={section.items}
                key={section.flow}
                onToggleExpanded={() => setIsExpanded((current) => !current)}
                segments={section.segments}
                title={flowSectionTitle(t, sectionReportTab, section.flow)}
                totalCents={sectionTotalCents}
                visibleItems={isExpanded ? section.items : section.items.slice(0, 5)}
              />
            );
          })}
        </section>
      ) : activeTab === 'member' ? (
        <ReportBreakdownPanel
          currencyCode={reportCurrency}
          entryCount={memberSingleItems[0]?.count ?? 0}
          heading={memberReportHeading(t, memberSingleFlow)}
          isExpanded={isExpanded}
          items={memberSingleItems}
          labelledBy={activeTabButtonId}
          onToggleExpanded={() => setIsExpanded((current) => !current)}
          panelId={activeTabPanelId}
          role="tabpanel"
          segments={memberSingleSegments}
          title={memberReportHeading(t, memberSingleFlow)}
          totalCents={memberSingleItems[0]?.amountCents ?? 0}
          visibleItems={isExpanded ? memberSingleItems : memberSingleItems.slice(0, 5)}
        />
      ) : (
        <ReportBreakdownPanel
          currencyCode={reportCurrency}
          entryCount={entryCount}
          heading={reportHeading(t, activeTab, flowFilter)}
          isExpanded={isExpanded}
          items={rankedItems}
          labelledBy={activeTabButtonId}
          onToggleExpanded={() => setIsExpanded((current) => !current)}
          panelId={activeTabPanelId}
          role="tabpanel"
          segments={segments}
          title={reportTitle(t, activeTab, flowFilter)}
          totalCents={totalCents}
          visibleItems={visibleItems}
        />
      )}
    </section>
  );
}

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

function buildDonutSegments(items: RankedItem[]): DonutSegment[] {
  const total = items.reduce((sum, item) => sum + Math.abs(item.amountCents), 0);
  const circumference = 2 * Math.PI * 70;
  let offset = 0;
  return items.slice(0, 8).map((item, index) => {
    const length = total > 0 ? (Math.abs(item.amountCents) / total) * circumference : 0;
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

function buildDimensionFlowSections(
  t: TFunction,
  entries: Entry[],
  tab: SectionReportTab,
  categories: Category[],
  accounts: Account[],
  timeMode: TimeMode,
  cursorDate: Date,
  currencyCode: string,
  rates: Map<string, number>,
): ReportFlowSection[] {
  return (['expense', 'income', 'balance'] as ReportFlow[]).map((flow) => {
    const items = buildDimensionFlowItems(t, entries, tab, flow, categories, accounts, timeMode, cursorDate, currencyCode, rates);
    return { flow, items, segments: buildDonutSegments(items) };
  });
}

function buildDimensionFlowItems(
  t: TFunction,
  entries: Entry[],
  tab: SectionReportTab,
  flow: ReportFlow,
  categories: Category[],
  accounts: Account[],
  timeMode: TimeMode,
  cursorDate: Date,
  currencyCode: string,
  rates: Map<string, number>,
): RankedItem[] {
  const rows = new Map<string, RankedItem>();
  const filtered = flow === 'balance'
    ? filterEntries(entries, 'all', timeMode, cursorDate)
    : filterEntries(entries, flow, timeMode, cursorDate);

  for (const entry of filtered) {
    const converted = convertEntryAmountCents(entry, currencyCode, rates);
    if (converted === null) {
      continue;
    }
    for (const key of dimensionKeys(t, entry, tab, categories, accounts)) {
      const current = rows.get(key.id) ?? { ...key, amountCents: 0, count: 0, percent: 0 };
      current.amountCents += flow === 'balance' && entry.type === 'expense' ? -converted : converted;
      current.count += 1;
      rows.set(key.id, current);
    }
  }

  const ranked = Array.from(rows.values()).sort((first, second) => Math.abs(second.amountCents) - Math.abs(first.amountCents));
  const denominator = flow === 'balance'
    ? ranked.reduce((sum, item) => sum + Math.abs(item.amountCents), 0)
    : Math.abs(ranked.reduce((sum, item) => sum + item.amountCents, 0));
  return ranked.map((item) => ({
    ...item,
    percent: denominator > 0 ? (Math.abs(item.amountCents) / denominator) * 100 : 0,
  }));
}

function buildMemberFlowSections(
  t: TFunction,
  entries: Entry[],
  members: BookMember[],
  timeMode: TimeMode,
  cursorDate: Date,
  currencyCode: string,
  rates: Map<string, number>,
): ReportFlowSection[] {
  return (['expense', 'income', 'balance'] as ReportFlow[]).map((flow) => {
    const items = buildMemberItems(t, entries, members, flow, timeMode, cursorDate, currencyCode, rates);
    return { flow, items, segments: buildDonutSegments(items.slice(1)) };
  });
}

function buildMemberItems(
  t: TFunction,
  entries: Entry[],
  members: BookMember[],
  flow: ReportFlow,
  timeMode: TimeMode,
  cursorDate: Date,
  currencyCode: string,
  rates: Map<string, number>,
): RankedItem[] {
  const rows = new Map<string, RankedItem>();
  const filtered = flow === 'balance'
    ? filterEntries(entries, 'all', timeMode, cursorDate)
    : filterEntries(entries, flow, timeMode, cursorDate);

  for (const entry of filtered) {
    const converted = convertEntryAmountCents(entry, currencyCode, rates);
    if (converted === null) {
      continue;
    }
    const key = entry.creatorUserId || 'no-member';
    const current = rows.get(key) ?? {
      id: key,
      label: memberLabel(t, entry, members),
      amountCents: 0,
      count: 0,
      percent: 0,
    };
    current.amountCents += flow === 'balance' && entry.type === 'expense' ? -converted : converted;
    current.count += 1;
    rows.set(key, current);
  }

  const ranked = Array.from(rows.values()).sort((first, second) => Math.abs(second.amountCents) - Math.abs(first.amountCents));
  const totalCents = ranked.reduce((sum, row) => sum + row.amountCents, 0);
  const totalCount = ranked.reduce((sum, row) => sum + row.count, 0);
  if (!ranked.length) {
    return [];
  }

  const denominator = flow === 'balance' ? ranked.reduce((sum, row) => sum + Math.abs(row.amountCents), 0) : Math.abs(totalCents);
  return [
    {
      id: 'shared-total',
      label: t('reports.member.sharedTotal'),
      amountCents: totalCents,
      count: totalCount,
      percent: denominator > 0 ? 100 : 0,
    },
    ...ranked.map((row) => ({
      ...row,
      percent: denominator > 0 ? (Math.abs(row.amountCents) / denominator) * 100 : 0,
    })),
  ];
}

function buildTrendData(
  entries: Entry[],
  timeMode: TimeMode,
  cursorDate: Date,
  currencyCode: string,
  rates: Map<string, number>,
): TrendData {
  const sourceRows = timeMode === 'year' ? yearBuckets(cursorDate) : monthBuckets(cursorDate);
  const rows: TrendBucket[] = sourceRows.map((row) => ({
    id: row.id,
    label: row.label,
    incomeCents: 0,
    expenseCents: 0,
    balanceCents: 0,
    count: 0,
  }));
  const index = new Map(rows.map((row) => [row.id, row]));
  for (const entry of filterEntries(entries, 'all', timeMode, cursorDate)) {
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
    if (entry.type === 'income') {
      current.incomeCents += amountCents;
      current.balanceCents += amountCents;
    } else {
      current.expenseCents += amountCents;
      current.balanceCents -= amountCents;
    }
    current.count += 1;
  }

  return {
    rows,
    incomeCents: rows.reduce((sum, row) => sum + row.incomeCents, 0),
    expenseCents: rows.reduce((sum, row) => sum + row.expenseCents, 0),
    balanceCents: rows.reduce((sum, row) => sum + row.balanceCents, 0),
    entryCount: rows.reduce((sum, row) => sum + row.count, 0),
  };
}

function memberLabel(t: TFunction, entry: Entry, members: BookMember[]): string {
  const importedMember = (entry as Entry & { member?: string }).member;
  if (importedMember) {
    return importedMember;
  }
  const member = members.find((item) => item.userId === entry.creatorUserId);
  return member?.displayName || entry.creatorUserId || t('reports.labels.noMember');
}

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

function startOfYear(date: Date): Date {
  return new Date(Date.UTC(date.getUTCFullYear(), 0, 1));
}

function startOfMonth(date: Date): Date {
  return new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), 1));
}

function formatPeriod(date: Date, mode: TimeMode): string {
  if (mode === 'year') {
    return String(date.getUTCFullYear());
  }

  return `${date.getUTCFullYear()}/${String(date.getUTCMonth() + 1).padStart(2, '0')}`;
}

function reportTitle(t: TFunction, tab: ReportTab, flowFilter: FlowFilter): string {
  return t('reports.panel.eyebrow', { dimension: t(`reports.tabs.${tab}`), flow: t(`common.flow.${flowFilter}`) });
}

function reportHeading(t: TFunction, tab: ReportTab, flowFilter: FlowFilter): string {
  const target = flowFilter === 'all' ? t('reports.cashflow') : t(`common.flow.${flowFilter}`);
  return t('reports.panel.heading', { dimension: t(`reports.tabs.${tab}`), target });
}

function flowSectionTitle(t: TFunction, tab: SectionReportTab, flow: ReportFlow): string {
  if (tab === 'member') {
    return memberReportHeading(t, flow);
  }

  return t('reports.panel.eyebrow', { dimension: t(`reports.tabs.${tab}`), flow: reportFlowLabel(t, flow) });
}

function flowSectionHeading(t: TFunction, tab: SectionReportTab, flow: ReportFlow): string {
  if (tab === 'member') {
    return memberReportHeading(t, flow);
  }

  return t('reports.panel.heading', { dimension: t(`reports.tabs.${tab}`), target: reportFlowLabel(t, flow) });
}

function reportFlowLabel(t: TFunction, flow: ReportFlow): string {
  return flow === 'balance' ? t('reports.flow.balance') : t(`common.flow.${flow}`);
}

function memberReportHeading(t: TFunction, flow: ReportFlow): string {
  return t(`reports.member.${flow}`);
}

function isSectionReportTab(tab: ReportTab): tab is SectionReportTab {
  return tab === 'category' || tab === 'subcategory' || tab === 'member';
}

function reportTabPath(tab: ReportTab): string {
  return `/reports/${tab}`;
}

function reportTabFromPath(pathname: string): ReportTab | null {
  const id = pathname.slice('/reports/'.length);
  return reportTabs.some((tab) => tab.id === id) ? id as ReportTab : null;
}
