import { ChevronDown, CircleDollarSign } from 'lucide-react';
import { useId } from 'react';
import { useTranslation } from 'react-i18next';
import { compactMoney, formatMoney } from '@/lib/money';
import { reportColors } from './reportColors';

export type RankedItem = {
  id: string;
  label: string;
  amountCents: number;
  count: number;
  percent: number;
};

export type DonutSegment = RankedItem & {
  color: string;
  dashArray: string;
  dashOffset: number;
};

export type TrendBucket = {
  id: string;
  label: string;
  incomeCents: number;
  expenseCents: number;
  balanceCents: number;
  count: number;
};

export type TrendData = {
  rows: TrendBucket[];
  incomeCents: number;
  expenseCents: number;
  balanceCents: number;
  entryCount: number;
};

const percent = new Intl.NumberFormat('en-US', {
  maximumFractionDigits: 2,
  minimumFractionDigits: 2,
});

// ReportBreakdownPanel receives ranked rows and renders one donut/list report section.
export function ReportBreakdownPanel({
  className = 'reportPanel',
  currencyCode,
  entryCount,
  heading,
  items,
  labelledBy,
  onToggleExpanded,
  panelId,
  role,
  segments,
  title,
  totalCents,
  visibleItems,
  isExpanded,
}: {
  className?: string;
  currencyCode: string;
  entryCount: number;
  heading: string;
  items: RankedItem[];
  labelledBy?: string;
  onToggleExpanded?: () => void;
  panelId?: string;
  role?: 'tabpanel';
  segments: DonutSegment[];
  title: string;
  totalCents: number;
  visibleItems: RankedItem[];
  isExpanded: boolean;
}) {
  const { t } = useTranslation();
  return (
    <section className={className} id={panelId} role={role} aria-labelledby={labelledBy}>
      <div className="reportPanelHeader">
        <div>
          <p className="eyebrow">{title}</p>
          <h3>{heading}</h3>
        </div>
        <span>{t('common.entriesCount', { value: entryCount })}</span>
      </div>

      <div className="reportBody">
        <DonutChart currencyCode={currencyCode} segments={segments} totalCents={totalCents} label={heading} />
        <RankedList currencyCode={currencyCode} items={visibleItems} totalCents={totalCents} />
      </div>

      {items.length > 5 && onToggleExpanded ? (
        <button className="expandReport" type="button" onClick={onToggleExpanded}>
          <span>{isExpanded ? t('reports.showLess') : t('reports.showAll')}</span>
          <ChevronDown size={16} className={isExpanded ? 'expandIconRotated' : ''} />
        </button>
      ) : null}
    </section>
  );
}

// TrendPanel receives period buckets and renders income, expense, and net movement.
export function TrendPanel({
  trend,
  panelId,
  labelledBy,
  currencyCode,
}: {
  trend: TrendData;
  panelId: string;
  labelledBy: string;
  currencyCode: string;
}) {
  const { t } = useTranslation();
  const metrics = [
    { id: 'income', label: t('reports.trend.income'), value: trend.incomeCents },
    { id: 'expense', label: t('reports.trend.expense'), value: trend.expenseCents },
    { id: 'balance', label: t('reports.trend.balance'), value: trend.balanceCents },
  ];

  return (
    <section className="reportPanel trendPanel" id={panelId} role="tabpanel" aria-labelledby={labelledBy}>
      <div className="reportPanelHeader">
        <div>
          <p className="eyebrow">{t('reports.tabs.trend')}</p>
          <h3>{t('reports.trend.heading')}</h3>
        </div>
        <span>{t('common.entriesCount', { value: trend.entryCount })}</span>
      </div>
      <div className="trendMetrics" aria-label={t('reports.trend.summary')}>
        {metrics.map((metric) => (
          <article className={`trendMetric trendMetric-${metric.id}`} key={metric.id}>
            <span>{metric.label}</span>
            <strong>{formatMoney(metric.value, currencyCode)}</strong>
          </article>
        ))}
      </div>
      <TrendChart rows={trend.rows} currencyCode={currencyCode} />
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
  const titleId = useId();
  const descriptionId = useId();
  const radius = 70;
  const stroke = 32;
  const center = 94;
  const hasData = segments.some((segment) => Math.abs(segment.amountCents) > 0);

  return (
    <figure className="donutFigure">
      <svg viewBox="0 0 188 188" role="img" aria-labelledby={`${titleId} ${descriptionId}`}>
        <title id={titleId}>{label}</title>
        <desc id={descriptionId}>
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
function RankedList({
  items,
  totalCents,
  currencyCode,
}: {
  items: RankedItem[];
  totalCents: number;
  currencyCode: string;
}) {
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
      {items.map((item, index) => {
        const barWidth = Math.abs(totalCents) > 0 || item.percent > 0 ? Math.max(3, Math.abs(item.percent)) : 0;
        return (
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
                <span style={{ width: `${barWidth}%`, background: reportColors[index % reportColors.length] }} />
              </div>
              <small>
                {t('common.entriesCount', { value: item.count })} <em>{percent.format(item.percent)}%</em>
              </small>
            </div>
          </li>
        );
      })}
    </ol>
  );
}

// TrendChart receives period buckets and renders income, expense, and balance lines.
function TrendChart({ rows, currencyCode }: { rows: TrendBucket[]; currencyCode: string }) {
  const { t } = useTranslation();
  const width = 640;
  const height = 260;
  const padding = { top: 24, right: 24, bottom: 42, left: 58 };
  const values = rows.flatMap((row) => [row.incomeCents, row.expenseCents, row.balanceCents]);
  const minValue = Math.min(0, ...values);
  const maxValue = Math.max(1, ...values);
  const xFor = (index: number) =>
    padding.left + (rows.length <= 1 ? 0 : (index / (rows.length - 1)) * (width - padding.left - padding.right));
  const yFor = (value: number) =>
    padding.top + ((maxValue - value) / (maxValue - minValue || 1)) * (height - padding.top - padding.bottom);
  const labelEvery = Math.max(1, Math.ceil(rows.length / 6));
  const series = [
    { id: 'income', label: t('reports.trend.income'), path: trendPath(rows, xFor, yFor, 'incomeCents') },
    { id: 'expense', label: t('reports.trend.expense'), path: trendPath(rows, xFor, yFor, 'expenseCents') },
    { id: 'balance', label: t('reports.trend.balance'), path: trendPath(rows, xFor, yFor, 'balanceCents') },
  ];
  const zeroY = yFor(0);

  return (
    <figure className="trendChart">
      <svg viewBox={`0 0 ${width} ${height}`} role="img" aria-labelledby="trendChartTitle trendChartDescription">
        <title id="trendChartTitle">{t('reports.trend.chartTitle')}</title>
        <desc id="trendChartDescription">{t('reports.trend.chartDesc')}</desc>
        {[0.25, 0.5, 0.75].map((ratio) => (
          <line
            className="trendGridLine"
            key={ratio}
            x1={padding.left}
            x2={width - padding.right}
            y1={padding.top + ratio * (height - padding.top - padding.bottom)}
            y2={padding.top + ratio * (height - padding.top - padding.bottom)}
          />
        ))}
        <line className="trendZeroLine" x1={padding.left} x2={width - padding.right} y1={zeroY} y2={zeroY} />
        {series.map((item) => (
          <path className={`trendLine trendLine-${item.id}`} d={item.path} key={item.id} />
        ))}
        {rows.map((row, index) =>
          index % labelEvery === 0 || index === rows.length - 1 ? (
            <text className="trendAxisLabel" key={row.id} x={xFor(index)} y={height - 14} textAnchor="middle">
              {row.label}
            </text>
          ) : null,
        )}
      </svg>
      <figcaption>
        {series.map((item) => (
          <span className={`trendLegend trendLegend-${item.id}`} key={item.id}>
            {item.label}
          </span>
        ))}
        <span>
          {t('reports.trend.range', {
            low: formatMoney(minValue, currencyCode),
            high: formatMoney(maxValue, currencyCode),
          })}
        </span>
      </figcaption>
    </figure>
  );
}

// trendPath receives buckets and accessors and returns an SVG line path.
function trendPath<K extends 'incomeCents' | 'expenseCents' | 'balanceCents'>(
  rows: TrendBucket[],
  xFor: (index: number) => number,
  yFor: (value: number) => number,
  key: K,
): string {
  return rows
    .map((row, index) => `${index === 0 ? 'M' : 'L'} ${xFor(index).toFixed(2)} ${yFor(row[key]).toFixed(2)}`)
    .join(' ');
}
