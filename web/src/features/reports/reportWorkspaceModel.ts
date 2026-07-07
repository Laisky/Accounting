import type { TFunction } from 'i18next';
import type { Account, BookMember, Category, Entry } from '@/lib/api/ledger';
import { convertEntryAmountCents } from '@/lib/money';
import type { DonutSegment, RankedItem, TrendBucket, TrendData } from './ReportVisuals';
import { reportColors } from './reportColors';

export type FlowFilter = 'all' | 'expense' | 'income';
type ReportFlow = 'expense' | 'income' | 'balance';
export type TimeMode = 'year' | 'month';
export type ReportTab = 'trend' | 'category' | 'subcategory' | 'member' | 'account' | 'merchant' | 'tag';
export type SectionReportTab = Extract<ReportTab, 'category' | 'subcategory' | 'member'>;
type ReportFlowSection = {
  flow: ReportFlow;
  items: RankedItem[];
  segments: DonutSegment[];
};

export const reportTabs: Array<{ id: ReportTab }> = [
  { id: 'trend' },
  { id: 'category' },
  { id: 'subcategory' },
  { id: 'member' },
  { id: 'account' },
  { id: 'merchant' },
  { id: 'tag' },
];

export function filterEntries(entries: Entry[], flowFilter: FlowFilter, timeMode: TimeMode, cursorDate: Date): Entry[] {
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

export function buildRankedItems(
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

export function buildDonutSegments(items: RankedItem[]): DonutSegment[] {
  const total = items.reduce((sum, item) => sum + Math.abs(item.amountCents), 0);
  const circumference = 2 * Math.PI * 70;
  let offset = 0;
  return items.slice(0, 8).map((item, index) => {
    const length = total > 0 ? (Math.abs(item.amountCents) / total) * circumference : 0;
    const segment = {
      ...item,
      color: reportColors[index % reportColors.length] ?? reportColors[0],
      dashArray: `${length} ${circumference - length}`,
      dashOffset: -offset,
    };
    offset += length;
    return segment;
  });
}

export function buildDimensionFlowSections(
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
    const items = buildDimensionFlowItems(
      t,
      entries,
      tab,
      flow,
      categories,
      accounts,
      timeMode,
      cursorDate,
      currencyCode,
      rates,
    );
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
  const filtered =
    flow === 'balance'
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

  const ranked = Array.from(rows.values()).sort(
    (first, second) => Math.abs(second.amountCents) - Math.abs(first.amountCents),
  );
  const denominator =
    flow === 'balance'
      ? ranked.reduce((sum, item) => sum + Math.abs(item.amountCents), 0)
      : Math.abs(ranked.reduce((sum, item) => sum + item.amountCents, 0));
  return ranked.map((item) => ({
    ...item,
    percent: denominator > 0 ? (Math.abs(item.amountCents) / denominator) * 100 : 0,
  }));
}

export function buildMemberFlowSections(
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
  const filtered =
    flow === 'balance'
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

  const ranked = Array.from(rows.values()).sort(
    (first, second) => Math.abs(second.amountCents) - Math.abs(first.amountCents),
  );
  const totalCents = ranked.reduce((sum, row) => sum + row.amountCents, 0);
  const totalCount = ranked.reduce((sum, row) => sum + row.count, 0);
  if (!ranked.length) {
    return [];
  }

  const denominator =
    flow === 'balance' ? ranked.reduce((sum, row) => sum + Math.abs(row.amountCents), 0) : Math.abs(totalCents);
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

export function buildTrendData(
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
    const key =
      timeMode === 'year'
        ? `${occurredAt.getUTCFullYear()}-${String(occurredAt.getUTCMonth() + 1).padStart(2, '0')}`
        : occurredAt.toISOString().slice(0, 10);
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

function dimensionKeys(
  t: TFunction,
  entry: Entry,
  tab: ReportTab,
  categories: Category[],
  accounts: Account[],
): Array<Pick<RankedItem, 'id' | 'label'>> {
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
      return [
        { id: account?.id ?? 'unassigned-account', label: account?.name ?? t('reports.labels.unassignedAccount') },
      ];
    case 'merchant':
      return [{ id: entry.merchant || 'no-merchant', label: entry.merchant || t('reports.labels.noMerchant') }];
    case 'tag':
      return entry.tags?.length
        ? entry.tags.map((tag) => ({ id: tag, label: tag }))
        : [{ id: 'no-tag', label: t('reports.labels.noTag') }];
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
    label: new Intl.DateTimeFormat('en-US', { month: 'short', timeZone: 'UTC' }).format(
      new Date(Date.UTC(year, month, 1)),
    ),
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

export function startOfYear(date: Date): Date {
  return new Date(Date.UTC(date.getUTCFullYear(), 0, 1));
}

export function startOfMonth(date: Date): Date {
  return new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), 1));
}

export function formatPeriod(date: Date, mode: TimeMode): string {
  if (mode === 'year') {
    return String(date.getUTCFullYear());
  }

  return `${date.getUTCFullYear()}/${String(date.getUTCMonth() + 1).padStart(2, '0')}`;
}

export function reportTitle(t: TFunction, tab: ReportTab, flowFilter: FlowFilter): string {
  return t('reports.panel.eyebrow', { dimension: t(`reports.tabs.${tab}`), flow: t(`common.flow.${flowFilter}`) });
}

export function reportHeading(t: TFunction, tab: ReportTab, flowFilter: FlowFilter): string {
  const target = flowFilter === 'all' ? t('reports.cashflow') : t(`common.flow.${flowFilter}`);
  return t('reports.panel.heading', { dimension: t(`reports.tabs.${tab}`), target });
}

export function flowSectionTitle(t: TFunction, tab: SectionReportTab, flow: ReportFlow): string {
  if (tab === 'member') {
    return memberReportHeading(t, flow);
  }

  return t('reports.panel.eyebrow', { dimension: t(`reports.tabs.${tab}`), flow: reportFlowLabel(t, flow) });
}

export function flowSectionHeading(t: TFunction, tab: SectionReportTab, flow: ReportFlow): string {
  if (tab === 'member') {
    return memberReportHeading(t, flow);
  }

  return t('reports.panel.heading', { dimension: t(`reports.tabs.${tab}`), target: reportFlowLabel(t, flow) });
}

function reportFlowLabel(t: TFunction, flow: ReportFlow): string {
  return flow === 'balance' ? t('reports.flow.balance') : t(`common.flow.${flow}`);
}

export function memberReportHeading(t: TFunction, flow: ReportFlow): string {
  return t(`reports.member.${flow}`);
}

export function isSectionReportTab(tab: ReportTab): tab is SectionReportTab {
  return tab === 'category' || tab === 'subcategory' || tab === 'member';
}

export function reportTabPath(tab: ReportTab): string {
  return `/reports/${tab}`;
}

export function isReportTab(value: string | undefined): value is ReportTab {
  return reportTabs.some((tab) => tab.id === value);
}
