import type { Entry, ExchangeRate } from './api/ledger';

export const supportedCurrencies = ['USD', 'EUR', 'CNY', 'CAD'] as const;

// buildRateIndex receives API exchange rates and returns a currency-to-units-per-USD map.
export function buildRateIndex(rates: ExchangeRate[]): Map<string, number> {
  const index = new Map<string, number>();
  for (const rate of rates) {
    const value = Number(rate.unitsPerUsd);
    if (!Number.isFinite(value) || value <= 0) {
      continue;
    }
    index.set(normalizeCurrencyCode(rate.currency), value);
  }

  return index;
}

// compactMoney receives cents and returns a dense center-label amount.
export function compactMoney(cents: number): string {
  return Math.abs(cents / 100).toLocaleString('en-US', { maximumFractionDigits: 0 });
}

// convertEntryAmountCents receives an entry, target currency, and rates and returns converted cents or null when FX is missing.
export function convertEntryAmountCents(entry: Entry, targetCurrency: string, rates: Map<string, number>): number | null {
  const sourceCurrency = normalizeCurrencyCode(entry.transactionCurrency || entry.bookReportingCurrency || targetCurrency);
  const normalizedTarget = normalizeCurrencyCode(targetCurrency);
  if (sourceCurrency === normalizedTarget) {
    return entry.amountCents;
  }

  const rate = parseExchangeRate(entry.exchangeRate);
  if (rate?.from === sourceCurrency && rate.to === normalizedTarget) {
    return Math.round(entry.amountCents * rate.rate);
  }
  if (rate?.from === normalizedTarget && rate.to === sourceCurrency) {
    return Math.round(entry.amountCents / rate.rate);
  }

  const sourceRate = rates.get(sourceCurrency);
  const targetRate = rates.get(normalizedTarget);
  if (!sourceRate || !targetRate) {
    return null;
  }

  return Math.round((entry.amountCents * targetRate) / sourceRate);
}

// formatMoney receives cents and an ISO currency code and returns localized currency text.
export function formatMoney(cents: number, currencyCode: string): string {
  return moneyFormatter(currencyCode).format(cents / 100);
}

// normalizeCurrencyCode receives user or API currency text and returns an uppercase code.
export function normalizeCurrencyCode(value: string): string {
  return value.trim().toUpperCase();
}

// parseExchangeRate receives exchange metadata and returns a normalized rate tuple.
function parseExchangeRate(value?: string): { from: string; to: string; rate: number } | null {
  const match = (value ?? '').trim().toUpperCase().match(/^([A-Z]{3})\/([A-Z]{3})=([0-9]+(?:\.[0-9]+)?)$/);
  if (!match) {
    return null;
  }

  const rate = Number(match[3]);
  if (!Number.isFinite(rate) || rate <= 0) {
    return null;
  }

  return {
    from: match[1],
    to: match[2],
    rate,
  };
}

// moneyFormatter receives an ISO currency code and returns a safe localized formatter.
function moneyFormatter(currencyCode: string): Intl.NumberFormat {
  try {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: currencyCode,
    });
  } catch {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
    });
  }
}
