import type { TFunction } from 'i18next';
import { type Entry } from '../../lib/api/ledger';

// entryDetailTitle receives an entry and returns the localized page title for its cashflow direction.
export function entryDetailTitle(entry: Entry | undefined, fallback: string, t: TFunction): string {
  if (!entry) {
    return fallback;
  }
  if (entry.type === 'income' || entry.type === 'refund' || entry.type === 'reimbursement' || entry.type === 'borrow' || entry.type === 'repayment') {
    return t('mobile.entryDetail.incomeTitle');
  }
  if (entry.type === 'transfer') {
    return t('mobile.entryDetail.transferTitle');
  }

  return t('mobile.entryDetail.expenseTitle');
}
