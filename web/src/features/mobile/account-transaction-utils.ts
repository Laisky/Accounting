import { type Entry } from '@/lib/api/ledger';

// accountEntries receives entries and returns only those that affect the selected account.
export function accountEntries(accountId: string, entries: Entry[]): Entry[] {
  return entries.filter((entry) => entry.accountId === accountId || entry.destinationAccountId === accountId);
}
