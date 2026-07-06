import { ChevronLeft, ChevronRight, Filter } from 'lucide-react';
import type { KeyboardEvent } from 'react';
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router';
import {
  fetchAccounts,
  fetchAllEntries,
  fetchBookMembers,
  fetchBooks,
  fetchCategories,
  fetchExchangeRates,
} from '@/lib/api/ledger';
import { buildRateIndex, convertEntryAmountCents, formatMoney } from '@/lib/money';
import { ReportBreakdownPanel, TrendPanel } from './ReportVisuals';

import {
  buildDimensionFlowSections,
  buildDonutSegments,
  buildMemberFlowSections,
  buildRankedItems,
  buildTrendData,
  filterEntries,
  flowSectionHeading,
  flowSectionTitle,
  formatPeriod,
  isSectionReportTab,
  memberReportHeading,
  reportHeading,
  reportTabPath,
  reportTabs,
  reportTitle,
  startOfMonth,
  startOfYear,
  type FlowFilter,
  type ReportData,
  type ReportTab,
  type SectionReportTab,
  type TimeMode,
} from './reportWorkspaceModel';
import './reports.css';
type ReportWorkspaceProps = {
  activeTab: ReportTab;
  baseCurrency?: string;
  refreshKey?: number;
};

// ReportWorkspace renders interactive reporting over real ledger entries.
export function ReportWorkspace({ activeTab, baseCurrency, refreshKey = 0 }: ReportWorkspaceProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [data, setData] = useState<ReportData>({
    books: [],
    members: [],
    accounts: [],
    categories: [],
    entries: [],
    rates: [],
  });
  const [selectedBookId, setSelectedBookId] = useState('');
  const [flowFilter, setFlowFilter] = useState<FlowFilter>('all');
  const [timeMode, setTimeMode] = useState<TimeMode>('month');
  const [cursorDate, setCursorDate] = useState(() => startOfMonth(new Date()));
  const [isExpanded, setIsExpanded] = useState(false);
  const [isFilterOpen, setIsFilterOpen] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState('');

  const selectedBook = data.books.find((book) => book.id === selectedBookId) ?? data.books[0];
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
  const conversionIssueCount = filteredEntries.filter(
    (entry) => convertEntryAmountCents(entry, reportCurrency, rateIndex) === null,
  ).length;
  const segments = useMemo(() => buildDonutSegments(rankedItems), [rankedItems]);
  const categorySections = useMemo(
    () =>
      buildDimensionFlowSections(
        t,
        data.entries,
        'category',
        data.categories,
        data.accounts,
        timeMode,
        cursorDate,
        reportCurrency,
        rateIndex,
      ),
    [cursorDate, data.accounts, data.categories, data.entries, rateIndex, reportCurrency, t, timeMode],
  );
  const subcategorySections = useMemo(
    () =>
      buildDimensionFlowSections(
        t,
        data.entries,
        'subcategory',
        data.categories,
        data.accounts,
        timeMode,
        cursorDate,
        reportCurrency,
        rateIndex,
      ),
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
  const allFlowSections =
    sectionReportTab === 'category'
      ? categorySections
      : sectionReportTab === 'subcategory'
        ? subcategorySections
        : memberSections;
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
    if (!nextTab) {
      return;
    }
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
            {data.books.length ? (
              data.books.map((book) => (
                <option key={book.id} value={book.id}>
                  {book.name}
                </option>
              ))
            ) : (
              <option>{t('common.noBookYet')}</option>
            )}
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
        <div
          className="reportFilterPanel"
          id="report-filter-panel"
          role="region"
          aria-label={t('reports.a11y.activeFilters')}
        >
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
        <section
          className="categorySplitPanel"
          id={activeTabPanelId}
          role="tabpanel"
          aria-labelledby={activeTabButtonId}
        >
          {allFlowSections.map((section) => {
            const isMemberSection = sectionReportTab === 'member';
            const sectionTotalCents = isMemberSection
              ? (section.items[0]?.amountCents ?? 0)
              : section.items.reduce((sum, item) => sum + item.amountCents, 0);
            const sectionEntryCount = isMemberSection
              ? (section.items[0]?.count ?? 0)
              : section.items.reduce((sum, item) => sum + item.count, 0);
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
