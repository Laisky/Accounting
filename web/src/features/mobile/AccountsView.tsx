import { ChevronDown, CreditCard, Eye, Globe2, Landmark, PiggyBank, TrendingUp } from 'lucide-react';
import { useMemo } from 'react';
import { type Account, type BookListItem } from '../../lib/api/ledger';
import { formatMoney, supportedCurrencies } from '../../lib/money';

type AccountsViewProps = {
  accounts: Account[];
  books: BookListItem[];
  currencyCode: string;
  isBusy: boolean;
  onPrepareAccount: () => void;
  onUpdateBookCurrency: (currency: string) => void;
  selectedBookId: string;
  setSelectedBookId: (value: string) => void;
};

type AccountSection = {
  id: string;
  label: string;
  count: number;
  totalCents: number;
  expanded: boolean;
  accounts: Account[];
};

// AccountsView receives account data and returns the Wacai-style account management tab.
export function AccountsView({
  accounts,
  books,
  currencyCode,
  isBusy,
  onPrepareAccount,
  onUpdateBookCurrency,
  selectedBookId,
  setSelectedBookId,
}: AccountsViewProps) {
  const sections = useMemo(() => buildAccountSections(accounts), [accounts]);
  const totalAssetsCents = accounts.reduce((sum, account) => sum + Math.max(0, account.openingBalanceCents), 0);
  const totalLiabilitiesCents = accounts.reduce((sum, account) => sum + Math.min(0, account.openingBalanceCents), 0);
  const netAssetsCents = totalAssetsCents + totalLiabilitiesCents;

  return (
    <section className="tabPanel accountPanel" aria-label="Accounts">
      <section className="accountHero" aria-label="Net assets">
        <div>
          <span>Net assets</span>
          <strong>
            {formatMoney(netAssetsCents, currencyCode)}
            <Eye size={20} />
          </strong>
        </div>
        <footer>
          <span>Total assets {formatMoney(totalAssetsCents, currencyCode)}</span>
          <i />
          <span>Total liabilities {formatMoney(Math.abs(totalLiabilitiesCents), currencyCode)}</span>
        </footer>
      </section>

      <div className="accountControls" aria-label="Account controls">
        <label>
          <span>Book</span>
          <select value={selectedBookId} onChange={(event) => setSelectedBookId(event.target.value)} disabled={!books.length}>
            {books.length ? books.map((book) => <option key={book.id} value={book.id}>{book.name}</option>) : <option>No book yet</option>}
          </select>
        </label>
        <label>
          <span>Base currency</span>
          <select value={currencyCode} onChange={(event) => onUpdateBookCurrency(event.target.value)} disabled={!books.length || isBusy}>
            {supportedCurrencies.map((currency) => <option key={currency} value={currency}>{currency}</option>)}
          </select>
        </label>
      </div>

      <div className="accountSectionList">
        {sections.map((section) => (
          <AccountSectionView key={section.id} currencyCode={currencyCode} section={section} />
        ))}
      </div>

      <button className="mobilePrimaryButton" type="button" disabled={isBusy} onClick={onPrepareAccount}>
        Prepare account
      </button>
    </section>
  );
}

// AccountSectionView receives one account section and returns its summary and expanded rows.
function AccountSectionView({ currencyCode, section }: { currencyCode: string; section: AccountSection }) {
  return (
    <article className={`accountSection ${section.expanded ? 'accountSectionOpen' : ''}`}>
      <header>
        <span>
          {section.label} <small>({section.count})</small>
        </span>
        <strong>{formatMoney(section.totalCents, currencyCode)}</strong>
        <ChevronDown size={18} />
      </header>
      {section.expanded ? (
        <ul>
          {section.accounts.length ? (
            section.accounts.map((account) => (
              <li key={account.id}>
                <span className="accountRowIcon">{accountIcon(account)}</span>
                <div>
                  <strong>{account.name}</strong>
                  <small>{account.type} / {account.currency}</small>
                </div>
                <b>{formatMoney(account.openingBalanceCents, account.currency)}</b>
              </li>
            ))
          ) : (
            <li className="accountEmptyRow">No accounts in this group.</li>
          )}
        </ul>
      ) : null}
    </article>
  );
}

// accountIcon receives an account and returns a category icon.
function accountIcon(account: Account) {
  const type = account.type.toLowerCase();
  if (type.includes('credit')) {
    return <CreditCard size={20} />;
  }
  if (type.includes('saving') || type.includes('debit')) {
    return <PiggyBank size={20} />;
  }
  if (type.includes('invest')) {
    return <TrendingUp size={20} />;
  }
  if (type.includes('online')) {
    return <Globe2 size={20} />;
  }

  return <Landmark size={20} />;
}

// buildAccountSections receives accounts and returns display groups matching the mobile account layout.
function buildAccountSections(accounts: Account[]): AccountSection[] {
  const cashAccounts = accounts.filter((account) => isCashAccount(account));
  const creditAccounts = accounts.filter((account) => isCreditAccount(account));
  const savingsAccounts = accounts.filter((account) => isSavingsAccount(account));
  const onlineAccounts = accounts.filter((account) => isOnlineAccount(account));
  const investmentAccounts = accounts.filter((account) => isInvestmentAccount(account));
  const storedAccounts = accounts.filter((account) => isStoredValueAccount(account));

  return [
    buildSection('cash', 'Cash', cashAccounts.length ? cashAccounts : accounts, true),
    buildSection('credit', 'Credit cards', creditAccounts, false),
    buildSection('savings', 'Savings and IOUs', savingsAccounts, false),
    buildSection('online', 'Online accounts', onlineAccounts, false),
    buildSection('investment', 'Investment accounts', investmentAccounts, false),
    buildSection('stored', 'Stored-value cards', storedAccounts, false),
  ];
}

// buildSection receives section parts and returns a normalized account section.
function buildSection(id: string, label: string, accounts: Account[], expanded: boolean): AccountSection {
  return {
    id,
    label,
    count: accounts.length,
    totalCents: accounts.reduce((sum, account) => sum + account.openingBalanceCents, 0),
    expanded,
    accounts,
  };
}

// isCashAccount receives an account and reports whether it belongs to the cash group.
function isCashAccount(account: Account): boolean {
  const text = `${account.type} ${account.name}`.toLowerCase();
  return text.includes('cash') || text.includes('checking') || text.includes('bank');
}

// isCreditAccount receives an account and reports whether it belongs to credit cards.
function isCreditAccount(account: Account): boolean {
  return `${account.type} ${account.name}`.toLowerCase().includes('credit');
}

// isSavingsAccount receives an account and reports whether it belongs to savings and IOUs.
function isSavingsAccount(account: Account): boolean {
  const text = `${account.type} ${account.name}`.toLowerCase();
  return text.includes('saving') || text.includes('debit') || text.includes('loan') || text.includes('iou');
}

// isOnlineAccount receives an account and reports whether it belongs to online accounts.
function isOnlineAccount(account: Account): boolean {
  const text = `${account.type} ${account.name}`.toLowerCase();
  return text.includes('online') || text.includes('wallet') || text.includes('paypal');
}

// isInvestmentAccount receives an account and reports whether it belongs to investment accounts.
function isInvestmentAccount(account: Account): boolean {
  const text = `${account.type} ${account.name}`.toLowerCase();
  return text.includes('invest') || text.includes('broker') || text.includes('stock');
}

// isStoredValueAccount receives an account and reports whether it belongs to stored-value cards.
function isStoredValueAccount(account: Account): boolean {
  const text = `${account.type} ${account.name}`.toLowerCase();
  return text.includes('stored') || text.includes('card');
}
