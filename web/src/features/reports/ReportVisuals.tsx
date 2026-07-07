import { ChevronDown, CircleDollarSign } from 'lucide-react';
import {
  type KeyboardEvent as ReactKeyboardEvent,
  type PointerEvent as ReactPointerEvent,
  useId,
  useRef,
  useState,
} from 'react';
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
  // activeId is the segment/row highlighted across the donut and ranked list (hover or keyboard focus).
  const [activeId, setActiveId] = useState<string | null>(null);
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
        <DonutChart
          activeId={activeId}
          currencyCode={currencyCode}
          label={heading}
          onActivate={setActiveId}
          segments={segments}
          totalCents={totalCents}
        />
        <RankedList
          activeId={activeId}
          currencyCode={currencyCode}
          items={visibleItems}
          onActivate={setActiveId}
          totalCents={totalCents}
        />
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

// DonutChart receives report segments and renders an interactive proportional donut chart.
function DonutChart({
  segments,
  totalCents,
  label,
  currencyCode,
  activeId,
  onActivate,
}: {
  segments: DonutSegment[];
  totalCents: number;
  label: string;
  currencyCode: string;
  activeId: string | null;
  onActivate: (id: string | null) => void;
}) {
  const { t } = useTranslation();
  const titleId = useId();
  const descriptionId = useId();
  const radius = 70;
  const stroke = 32;
  const center = 94;
  const hasData = segments.some((segment) => Math.abs(segment.amountCents) > 0);
  const activeSegment = segments.find((segment) => segment.id === activeId) ?? null;
  const centerLabel = activeSegment ? truncateLabel(activeSegment.label) : t('common.total');
  const centerAmountCents = activeSegment ? activeSegment.amountCents : totalCents;

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
            className="donutSegment"
            key={segment.id}
            cx={center}
            cy={center}
            r={radius}
            fill="none"
            stroke={segment.color}
            strokeDasharray={segment.dashArray}
            strokeDashoffset={segment.dashOffset}
            strokeLinecap="butt"
            strokeWidth={activeSegment?.id === segment.id ? stroke + 7 : stroke}
            opacity={activeSegment && activeSegment.id !== segment.id ? 0.35 : 1}
            onPointerEnter={() => onActivate(segment.id)}
            onPointerLeave={() => onActivate(null)}
          />
        ))}
        <text x="94" y="84" textAnchor="middle">
          {centerLabel}
        </text>
        <text className="donutAmount" x="94" y="112" textAnchor="middle">
          {compactMoney(centerAmountCents)}
        </text>
        {activeSegment ? (
          <text className="donutPercent" x="94" y="130" textAnchor="middle">
            {percent.format(activeSegment.percent)}%
          </text>
        ) : null}
      </svg>
      {hasData ? (
        <figcaption>
          {segments.slice(0, 4).map((segment) => (
            <button
              type="button"
              className={`donutLegend${activeSegment && activeSegment.id !== segment.id ? ' donutLegendDim' : ''}`}
              key={segment.id}
              onPointerEnter={() => onActivate(segment.id)}
              onPointerLeave={() => onActivate(null)}
              onFocus={() => onActivate(segment.id)}
              onBlur={() => onActivate(null)}
            >
              <i style={{ background: segment.color }} />
              {segment.label} {percent.format(segment.percent)}%
            </button>
          ))}
        </figcaption>
      ) : (
        <figcaption>{t('reports.donut.emptyCaption')}</figcaption>
      )}
    </figure>
  );
}

// truncateLabel shortens a segment label so it fits inside the donut's center hole.
function truncateLabel(label: string): string {
  return label.length > 14 ? `${label.slice(0, 13)}…` : label;
}

// RankedList receives ranked report rows and returns an interactive ranked amount list.
function RankedList({
  items,
  totalCents,
  currencyCode,
  activeId,
  onActivate,
}: {
  items: RankedItem[];
  totalCents: number;
  currencyCode: string;
  activeId: string | null;
  onActivate: (id: string | null) => void;
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
        const isActive = item.id === activeId;
        const isDim = activeId != null && !isActive;
        return (
          <li
            className={`rankRow${isActive ? ' rankRowActive' : ''}${isDim ? ' rankRowDim' : ''}`}
            key={item.id}
            onPointerEnter={() => onActivate(item.id)}
            onPointerLeave={() => onActivate(null)}
          >
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

type TrendSeriesId = 'income' | 'expense' | 'balance';
type TrendValueKey = 'incomeCents' | 'expenseCents' | 'balanceCents';
const trendSeriesMeta: { id: TrendSeriesId; key: TrendValueKey; labelKey: string }[] = [
  { id: 'income', key: 'incomeCents', labelKey: 'reports.trend.income' },
  { id: 'expense', key: 'expenseCents', labelKey: 'reports.trend.expense' },
  { id: 'balance', key: 'balanceCents', labelKey: 'reports.trend.balance' },
];

// TrendChart receives period buckets and renders a scrubbable income/expense/balance timeline with legend toggles.
function TrendChart({ rows, currencyCode }: { rows: TrendBucket[]; currencyCode: string }) {
  const { t } = useTranslation();
  const trackRef = useRef<HTMLDivElement>(null);
  const [hidden, setHidden] = useState<ReadonlySet<TrendSeriesId>>(() => new Set());
  const [scrubIndex, setScrubIndex] = useState<number | null>(null);
  const [isActive, setIsActive] = useState(false);

  const width = 640;
  const height = 260;
  const padding = { top: 24, right: 24, bottom: 42, left: 58 };
  const count = rows.length;
  const active = count > 0 ? Math.min(Math.max(scrubIndex ?? count - 1, 0), count - 1) : 0;
  const activeRow = rows[active];

  const series = trendSeriesMeta.map((meta) => ({ ...meta, label: t(meta.labelKey), off: hidden.has(meta.id) }));
  const visibleSeries = series.filter((item) => !item.off);
  const scaleSeries = visibleSeries.length ? visibleSeries : series;
  const values = rows.flatMap((row) => scaleSeries.map((item) => row[item.key]));
  const minValue = Math.min(0, ...values);
  const maxValue = Math.max(1, ...values);
  const xFor = (index: number) =>
    padding.left + (count <= 1 ? 0 : (index / (count - 1)) * (width - padding.left - padding.right));
  const yFor = (value: number) =>
    padding.top + ((maxValue - value) / (maxValue - minValue || 1)) * (height - padding.top - padding.bottom);
  const labelEvery = Math.max(1, Math.ceil(count / 6));
  const zeroY = yFor(0);
  const activeXPercent = (xFor(active) / width) * 100;
  const tooltipLeft = Math.min(86, Math.max(14, activeXPercent));
  const valueText = activeRow
    ? t('reports.trend.point', {
        period: activeRow.label,
        income: formatMoney(activeRow.incomeCents, currencyCode),
        expense: formatMoney(activeRow.expenseCents, currencyCode),
        balance: formatMoney(activeRow.balanceCents, currencyCode),
      })
    : '';

  // indexFromClientX receives a pointer x coordinate and returns the nearest period index.
  function indexFromClientX(clientX: number): number {
    const rect = trackRef.current?.getBoundingClientRect();
    if (!rect || rect.width === 0 || count <= 1) {
      return count > 0 ? count - 1 : 0;
    }

    const viewBoxX = ((clientX - rect.left) / rect.width) * width;
    const ratio = (viewBoxX - padding.left) / (width - padding.left - padding.right);
    return Math.round(Math.min(1, Math.max(0, ratio)) * (count - 1));
  }

  // startScrub activates the crosshair at the period nearest the pointer.
  function startScrub(clientX: number) {
    setIsActive(true);
    setScrubIndex(indexFromClientX(clientX));
  }

  // endScrub clears the crosshair and returns the chart to its resting state.
  function endScrub() {
    setIsActive(false);
    setScrubIndex(null);
  }

  // handlePointerDown captures the pointer and scrubs to the contact position.
  function handlePointerDown(event: ReactPointerEvent<HTMLDivElement>) {
    event.currentTarget.setPointerCapture?.(event.pointerId);
    startScrub(event.clientX);
  }

  // handlePointerLeave resets when the mouse leaves; touch/pen reset on release instead.
  function handlePointerLeave(event: ReactPointerEvent<HTMLDivElement>) {
    if (event.pointerType === 'mouse') {
      endScrub();
    }
  }

  // handlePointerEnd resets touch/pen input on release or cancel (e.g. a vertical page scroll takes over).
  function handlePointerEnd(event: ReactPointerEvent<HTMLDivElement>) {
    if (event.pointerType !== 'mouse') {
      endScrub();
    }
  }

  // handleKeyDown moves the active period with arrow, Home, and End keys.
  function handleKeyDown(event: ReactKeyboardEvent<HTMLDivElement>) {
    let next: number;
    switch (event.key) {
      case 'ArrowRight':
      case 'ArrowUp':
        next = Math.min(count - 1, active + 1);
        break;
      case 'ArrowLeft':
      case 'ArrowDown':
        next = Math.max(0, active - 1);
        break;
      case 'Home':
        next = 0;
        break;
      case 'End':
        next = count - 1;
        break;
      default:
        return;
    }

    event.preventDefault();
    setIsActive(true);
    setScrubIndex(next);
  }

  // toggleSeries shows or hides one trend line from the legend.
  function toggleSeries(id: TrendSeriesId) {
    setHidden((current) => {
      const next = new Set(current);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }

      return next;
    });
  }

  return (
    <figure className="trendChart">
      <div className="trendChartPlot">
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
          {visibleSeries.map((item) => (
            <path
              className={`trendLine trendLine-${item.id}`}
              d={trendPath(rows, xFor, yFor, item.key)}
              key={item.id}
            />
          ))}
          {rows.map((row, index) =>
            index % labelEvery === 0 || index === count - 1 ? (
              <text className="trendAxisLabel" key={row.id} x={xFor(index)} y={height - 14} textAnchor="middle">
                {row.label}
              </text>
            ) : null,
          )}
          {isActive && activeRow ? (
            <>
              <line
                className="trendCrosshair"
                x1={xFor(active)}
                x2={xFor(active)}
                y1={padding.top}
                y2={height - padding.bottom}
              />
              {visibleSeries.map((item) => (
                <circle
                  className={`trendDot trendDot-${item.id}`}
                  cx={xFor(active)}
                  cy={yFor(activeRow[item.key])}
                  r={5}
                  key={item.id}
                />
              ))}
            </>
          ) : null}
        </svg>

        {isActive && activeRow ? (
          <div className="trendTooltip" style={{ left: `${tooltipLeft}%` }} aria-hidden="true">
            <span className="trendTooltipPeriod">{activeRow.label}</span>
            {visibleSeries.map((item) => (
              <span className={`trendTooltipRow trendTooltipRow-${item.id}`} key={item.id}>
                <em>{item.label}</em>
                <b>{formatMoney(activeRow[item.key], currencyCode)}</b>
              </span>
            ))}
          </div>
        ) : null}

        {count > 0 ? (
          <div
            ref={trackRef}
            className="trendChartTrack"
            role="slider"
            tabIndex={0}
            aria-label={t('reports.trend.scrubber')}
            aria-orientation="horizontal"
            aria-valuemin={0}
            aria-valuemax={Math.max(0, count - 1)}
            aria-valuenow={active}
            aria-valuetext={valueText}
            onKeyDown={handleKeyDown}
            onFocus={() => setIsActive(true)}
            onBlur={endScrub}
            onPointerDown={handlePointerDown}
            onPointerMove={(event) => startScrub(event.clientX)}
            onPointerLeave={handlePointerLeave}
            onPointerUp={handlePointerEnd}
            onPointerCancel={handlePointerEnd}
          />
        ) : null}
      </div>

      <figcaption>
        {series.map((item) => (
          <button
            type="button"
            className={`trendLegend trendLegend-${item.id}${item.off ? ' trendLegendOff' : ''}`}
            key={item.id}
            aria-pressed={!item.off}
            onClick={() => toggleSeries(item.id)}
          >
            {item.label}
          </button>
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
