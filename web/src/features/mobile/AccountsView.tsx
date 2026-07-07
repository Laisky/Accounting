import { ChevronDown, CreditCard, Eye, Globe2, Landmark, PiggyBank, TrendingUp } from 'lucide-react';
import { type FormEvent, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router';
import { useBook } from '@/contexts/BookContext';
import { useNotice } from '@/contexts/NoticeContext';
import {
  useAccountGroupsQuery,
  useAccountsQuery,
  useCreateAccountGroupMutation,
  useCreateAccountMutation,
  useUpdateAccountGroupMutation,
} from '@/hooks/useAccounts';
import { useBookMembersQuery } from '@/hooks/useBookMembers';
import { useCreateBookMutation, useUpdateBookMutation } from '@/hooks/useBooks';
import { usePrepareStarterAccount } from '@/hooks/usePrepareStarterAccount';
import { type Account, type AccountGroup, type BookListItem, type BookMember } from '@/lib/api/ledger';
import { convertCurrencyAmountCents, formatMoney, supportedCurrencies } from '@/lib/money';
import './accounts.css';

export type AccountCreateInput = {
  name: string;
  type: string;
  currency: string;
  openingBalanceCents: number;
};

type AccountSection = {
  id: string;
  label: string;
  count: number;
  totalCents: number;
  expanded: boolean;
  accounts: Account[];
};

// AccountsView owns the account management tab, fetching book and account data through hooks
// and running create/update mutations that invalidate the shared Query cache.
export function AccountsView() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { books, selectedBook, selectedBookId, setSelectedBookId, bookCurrency, displayCurrency, rateIndex } =
    useBook();
  const { notifyError, notifyStatus } = useNotice();
  const accountsQuery = useAccountsQuery();
  const accounts = useMemo(
    () =>
      (accountsQuery.data ?? []).filter((account) => !selectedBook || account.sharedBookIds?.includes(selectedBook.id)),
    [accountsQuery.data, selectedBook],
  );
  const groups = useAccountGroupsQuery().data ?? [];
  const members = useBookMembersQuery(selectedBook?.id).data ?? [];
  const createBook = useCreateBookMutation();
  const createGroup = useCreateAccountGroupMutation();
  const createAccountMutation = useCreateAccountMutation();
  const updateGroup = useUpdateAccountGroupMutation();
  const updateBookMutation = useUpdateBookMutation();
  const starter = usePrepareStarterAccount();
  const bookCurrencyCode = bookCurrency;
  const displayCurrencyCode = displayCurrency;
  const isBusy =
    createBook.isPending ||
    createGroup.isPending ||
    createAccountMutation.isPending ||
    updateGroup.isPending ||
    updateBookMutation.isPending ||
    starter.isPending;

  const [accountName, setAccountName] = useState('');
  const [accountType, setAccountType] = useState('cash');
  const [accountCurrency, setAccountCurrency] = useState(bookCurrency);
  const [openingBalance, setOpeningBalance] = useState('');
  const [expandedSectionIds, setExpandedSectionIds] = useState<ReadonlySet<string>>(() => new Set(['cash']));
  const primaryGroup = groups[0];

  // onOpenAccount navigates to an account's transaction detail route.
  function onOpenAccount(accountId: string) {
    navigate(`/accounts/${encodeURIComponent(accountId)}/transactions`);
  }

  // onPrepareAccount bootstraps a starter book, group, and cash account for an empty ledger.
  function onPrepareAccount() {
    void starter.prepare();
  }

  // onCreateAccount creates the account, seeding a book and group when the ledger is empty.
  async function onCreateAccount(input: AccountCreateInput) {
    try {
      const book =
        selectedBook ??
        (await createBook.mutateAsync({ name: 'Household', reportingCurrency: displayCurrency || input.currency }));
      const group = groups[0] ?? (await createGroup.mutateAsync('Everyday'));
      await createAccountMutation.mutateAsync({
        groupId: group.id,
        name: input.name,
        type: input.type,
        currency: input.currency,
        sharedBookIds: [book.id],
        openingBalanceCents: input.openingBalanceCents,
      });
      setSelectedBookId(book.id);
      notifyStatus(t('mobile.status.accountCreated'));
    } catch (error) {
      notifyError(t('mobile.error.accountCreateFailed'));
      throw error;
    }
  }

  // onUpdateAccountGroupName renames the primary account group.
  async function onUpdateAccountGroupName(groupId: string, name: string) {
    try {
      await updateGroup.mutateAsync({ groupId, input: { name } });
      notifyStatus(t('mobile.status.accountGroupUpdated'));
    } catch {
      notifyError(t('mobile.error.accountGroupUpdateFailed'));
    }
  }

  // onUpdateBookName renames the selected book.
  async function onUpdateBookName(name: string) {
    if (!selectedBook) {
      return;
    }
    try {
      await updateBookMutation.mutateAsync({ bookId: selectedBook.id, input: { name } });
      notifyStatus(t('mobile.status.bookUpdated'));
    } catch {
      notifyError(t('mobile.error.bookUpdateFailed'));
    }
  }

  // onUpdateBookCurrency changes the selected book's reporting currency.
  async function onUpdateBookCurrency(currency: string) {
    if (!selectedBook) {
      return;
    }
    try {
      await updateBookMutation.mutateAsync({ bookId: selectedBook.id, input: { reportingCurrency: currency } });
      notifyStatus(t('mobile.status.baseCurrencyUpdated'));
    } catch {
      notifyError(t('mobile.error.baseCurrencyFailed'));
    }
  }
  const sections = useMemo(
    () => buildAccountSections(accounts, expandedSectionIds, t, displayCurrencyCode, rateIndex),
    [accounts, displayCurrencyCode, expandedSectionIds, rateIndex, t],
  );
  const totalAssetsCents = accounts.reduce(
    (sum, account) => sum + Math.max(0, accountDisplayBalanceCents(account, displayCurrencyCode, rateIndex)),
    0,
  );
  const totalLiabilitiesCents = accounts.reduce(
    (sum, account) => sum + Math.min(0, accountDisplayBalanceCents(account, displayCurrencyCode, rateIndex)),
    0,
  );
  const netAssetsCents = totalAssetsCents + totalLiabilitiesCents;
  const normalizedName = accountName.trim();
  const canCreateAccount = Boolean(normalizedName) && !isBusy;

  // handleCreateSubmit receives a form event, creates an account, and resets local fields after success.
  async function handleCreateSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!normalizedName) {
      return;
    }

    try {
      await onCreateAccount({
        name: normalizedName,
        type: accountType,
        currency: accountCurrency,
        openingBalanceCents: parseMajorAmountToCents(openingBalance),
      });
    } catch {
      return;
    }

    setAccountName('');
    setOpeningBalance('');
  }

  // handleToggleSection receives a section id and toggles that account section's row visibility.
  function handleToggleSection(sectionId: string) {
    setExpandedSectionIds((current) => {
      const next = new Set(current);
      if (next.has(sectionId)) {
        next.delete(sectionId);
      } else {
        next.add(sectionId);
      }

      return next;
    });
  }

  return (
    <section className="tabPanel accountPanel" aria-label={t('mobile.nav.accounts')}>
      {/* Left column at >=1024px; transparent (display:contents) below, so mobile is unchanged. */}
      <div className="accountPrimaryColumn">
        <section className="accountHero" aria-label={t('mobile.accounts.netAssets')}>
          <div>
            <span>{t('mobile.accounts.netAssets')}</span>
            <strong>
              {formatMoney(netAssetsCents, displayCurrencyCode)}
              <Eye size={20} />
            </strong>
          </div>
          <footer>
            <span>
              {t('mobile.accounts.totalAssets', { amount: formatMoney(totalAssetsCents, displayCurrencyCode) })}
            </span>
            <i />
            <span>
              {t('mobile.accounts.totalLiabilities', {
                amount: formatMoney(Math.abs(totalLiabilitiesCents), displayCurrencyCode),
              })}
            </span>
          </footer>
        </section>

        <BookSettingsView
          key={selectedBook?.id ?? 'empty-book-settings'}
          books={books}
          currencyCode={bookCurrencyCode}
          isBusy={isBusy}
          onUpdateBookCurrency={onUpdateBookCurrency}
          onUpdateBookName={onUpdateBookName}
          selectedBook={selectedBook}
          selectedBookId={selectedBookId}
          setSelectedBookId={setSelectedBookId}
        />

        <AccountGroupSettingsView
          key={primaryGroup?.id ?? 'empty-account-group-settings'}
          group={primaryGroup}
          isBusy={isBusy}
          onUpdateAccountGroupName={onUpdateAccountGroupName}
        />

        <form
          className="accountCreateForm"
          aria-label={t('mobile.accounts.createAccount')}
          onSubmit={handleCreateSubmit}
        >
          <label>
            <span>{t('mobile.accounts.accountName')}</span>
            <input
              value={accountName}
              onChange={(event) => setAccountName(event.target.value)}
              placeholder={t('mobile.accounts.accountNamePlaceholder')}
              maxLength={80}
            />
          </label>
          <label>
            <span>{t('mobile.accounts.type')}</span>
            <select value={accountType} onChange={(event) => setAccountType(event.target.value)}>
              <option value="cash">{t('mobile.accounts.types.cash')}</option>
              <option value="credit_card">{t('mobile.accounts.types.creditCard')}</option>
              <option value="savings">{t('mobile.accounts.types.savings')}</option>
              <option value="loan">{t('mobile.accounts.types.loan')}</option>
              <option value="payment_platform">{t('mobile.accounts.types.paymentPlatform')}</option>
              <option value="investment">{t('mobile.accounts.types.investment')}</option>
            </select>
          </label>
          <label>
            <span>{t('mobile.accounts.currency')}</span>
            <select value={accountCurrency} onChange={(event) => setAccountCurrency(event.target.value)}>
              {supportedCurrencies.map((currency) => (
                <option key={currency} value={currency}>
                  {currency}
                </option>
              ))}
            </select>
          </label>
          <label>
            <span>{t('mobile.accounts.openingBalance')}</span>
            <input
              inputMode="decimal"
              value={openingBalance}
              onChange={(event) => setOpeningBalance(event.target.value)}
              placeholder="0.00"
            />
          </label>
          <button type="submit" disabled={!canCreateAccount}>
            {t('mobile.accounts.createAccount')}
          </button>
        </form>
      </div>

      <div className="accountSectionList">
        <BookMembersView members={members} />
        {sections.map((section) => (
          <AccountSectionView
            key={section.id}
            currencyCode={displayCurrencyCode}
            onOpenAccount={onOpenAccount}
            onToggle={handleToggleSection}
            section={section}
          />
        ))}
      </div>

      <button className="mobilePrimaryButton" type="button" disabled={isBusy} onClick={onPrepareAccount}>
        {t('mobile.accounts.prepareAccount')}
      </button>
    </section>
  );
}

// BookMembersView receives selected-book members and returns a read-only membership list.
function BookMembersView({ members }: { members: BookMember[] }) {
  const { t } = useTranslation();
  return (
    <article className="accountSection accountSectionOpen" aria-label={t('mobile.accounts.bookMembers')}>
      <header className="accountSectionHeader">
        <span>
          {t('mobile.accounts.bookMembers')} <small>({members.length})</small>
        </span>
        <strong>{members.length}</strong>
        <ChevronDown size={18} />
      </header>
      <ul>
        {members.length ? (
          members.map((member) => (
            <li key={`${member.bookId}:${member.userId}`}>
              <span className="accountRowIcon">
                <Eye size={20} />
              </span>
              <div>
                <strong>{member.displayName || member.userId}</strong>
                <small>{member.userId}</small>
              </div>
              <b>{member.role}</b>
            </li>
          ))
        ) : (
          <li className="accountEmptyRow">{t('mobile.accounts.noMembers')}</li>
        )}
      </ul>
    </article>
  );
}

// AccountGroupSettingsView receives the primary group and returns an editable group settings form.
function AccountGroupSettingsView({
  group,
  isBusy,
  onUpdateAccountGroupName,
}: {
  group?: AccountGroup;
  isBusy: boolean;
  onUpdateAccountGroupName: (groupId: string, name: string) => Promise<void>;
}) {
  const { t } = useTranslation();
  const [groupName, setGroupName] = useState(group?.name ?? '');
  const normalizedGroupName = groupName.trim();
  const canSaveGroupName =
    Boolean(group) && Boolean(normalizedGroupName) && normalizedGroupName !== group?.name && !isBusy;

  // handleGroupNameSubmit receives a form event, updates the group name, and keeps the draft normalized.
  async function handleGroupNameSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!group || !canSaveGroupName) {
      return;
    }

    await onUpdateAccountGroupName(group.id, normalizedGroupName);
    setGroupName(normalizedGroupName);
  }

  return (
    <form className="accountControls" aria-label={t('mobile.accounts.groupSettings')} onSubmit={handleGroupNameSubmit}>
      <label className="bookNameField">
        <span>{t('mobile.accounts.groupName')}</span>
        <input
          value={groupName}
          onChange={(event) => setGroupName(event.target.value)}
          maxLength={120}
          disabled={!group || isBusy}
        />
      </label>
      <button type="submit" disabled={!canSaveGroupName}>
        {t('mobile.accounts.saveGroup')}
      </button>
    </form>
  );
}

// BookSettingsView receives book controls and returns the editable selected-book settings form.
function BookSettingsView({
  books,
  currencyCode,
  isBusy,
  onUpdateBookCurrency,
  onUpdateBookName,
  selectedBook,
  selectedBookId,
  setSelectedBookId,
}: {
  books: BookListItem[];
  currencyCode: string;
  isBusy: boolean;
  onUpdateBookCurrency: (currency: string) => void;
  onUpdateBookName: (name: string) => Promise<void>;
  selectedBook?: BookListItem;
  selectedBookId: string;
  setSelectedBookId: (value: string) => void;
}) {
  const { t } = useTranslation();
  const [bookName, setBookName] = useState(selectedBook?.name ?? '');
  const normalizedBookName = bookName.trim();
  const canSaveBookName =
    Boolean(selectedBook) && Boolean(normalizedBookName) && normalizedBookName !== selectedBook?.name && !isBusy;

  // handleBookNameSubmit receives a form event, updates the selected book name, and keeps the draft in sync.
  async function handleBookNameSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!canSaveBookName) {
      return;
    }

    await onUpdateBookName(normalizedBookName);
    setBookName(normalizedBookName);
  }

  return (
    <form className="accountControls" aria-label={t('mobile.accounts.bookSettings')} onSubmit={handleBookNameSubmit}>
      <label>
        <span>{t('mobile.accounts.book')}</span>
        <select
          value={selectedBookId}
          onChange={(event) => setSelectedBookId(event.target.value)}
          disabled={!books.length}
        >
          {books.length ? (
            books.map((book) => (
              <option key={book.id} value={book.id}>
                {book.name}
              </option>
            ))
          ) : (
            <option>{t('mobile.accounts.noBook')}</option>
          )}
        </select>
      </label>
      <label>
        <span>{t('mobile.accounts.baseCurrency')}</span>
        <select
          value={currencyCode}
          onChange={(event) => onUpdateBookCurrency(event.target.value)}
          disabled={!books.length || isBusy}
        >
          {supportedCurrencies.map((currency) => (
            <option key={currency} value={currency}>
              {currency}
            </option>
          ))}
        </select>
      </label>
      <label className="bookNameField">
        <span>{t('mobile.accounts.bookName')}</span>
        <input
          value={bookName}
          onChange={(event) => setBookName(event.target.value)}
          maxLength={120}
          disabled={!selectedBook || isBusy}
        />
      </label>
      <button type="submit" disabled={!canSaveBookName}>
        {t('mobile.accounts.saveBook')}
      </button>
    </form>
  );
}

// AccountSectionView receives one account section and returns its summary and expanded rows.
function AccountSectionView({
  currencyCode,
  onOpenAccount,
  onToggle,
  section,
}: {
  currencyCode: string;
  onOpenAccount: (accountId: string) => void;
  onToggle: (sectionId: string) => void;
  section: AccountSection;
}) {
  const { t } = useTranslation();
  const rowListId = `account-section-${section.id}`;

  return (
    <article className={`accountSection ${section.expanded ? 'accountSectionOpen' : ''}`}>
      <header className="accountSectionHeader">
        <button
          className="accountSectionToggle"
          type="button"
          aria-controls={rowListId}
          aria-expanded={section.expanded}
          onClick={() => onToggle(section.id)}
        >
          <span>
            {section.label} <small>({section.count})</small>
          </span>
          <strong>{formatMoney(section.totalCents, currencyCode)}</strong>
          <ChevronDown size={18} aria-hidden="true" />
        </button>
      </header>
      {section.expanded ? (
        <ul id={rowListId}>
          {section.accounts.length ? (
            section.accounts.map((account) => (
              <li key={account.id}>
                <button
                  type="button"
                  className="accountRowButton"
                  aria-label={t('mobile.accounts.openAccount', { name: account.name })}
                  onClick={() => onOpenAccount(account.id)}
                >
                  <span className="accountRowIcon">{accountIcon(account)}</span>
                  <div>
                    <strong>{account.name}</strong>
                    <small>
                      {account.type} / {account.currency}
                    </small>
                  </div>
                  <b>{formatMoney(account.openingBalanceCents, account.currency)}</b>
                </button>
              </li>
            ))
          ) : (
            <li className="accountEmptyRow">{t('mobile.accounts.emptyGroup')}</li>
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
function buildAccountSections(
  accounts: Account[],
  expandedSectionIds: ReadonlySet<string>,
  t: (key: string) => string,
  displayCurrencyCode: string,
  rates: Map<string, number>,
): AccountSection[] {
  const cashAccounts = accounts.filter((account) => isCashAccount(account));
  const creditAccounts = accounts.filter((account) => isCreditAccount(account));
  const savingsAccounts = accounts.filter((account) => isSavingsAccount(account));
  const onlineAccounts = accounts.filter((account) => isOnlineAccount(account));
  const investmentAccounts = accounts.filter((account) => isInvestmentAccount(account));
  const storedAccounts = accounts.filter((account) => !isCreditAccount(account) && isStoredValueAccount(account));

  return [
    buildSection(
      'cash',
      t('mobile.accounts.sections.cash'),
      cashAccounts.length ? cashAccounts : accounts,
      expandedSectionIds.has('cash'),
      displayCurrencyCode,
      rates,
    ),
    buildSection(
      'credit',
      t('mobile.accounts.sections.credit'),
      creditAccounts,
      expandedSectionIds.has('credit'),
      displayCurrencyCode,
      rates,
    ),
    buildSection(
      'savings',
      t('mobile.accounts.sections.savings'),
      savingsAccounts,
      expandedSectionIds.has('savings'),
      displayCurrencyCode,
      rates,
    ),
    buildSection(
      'online',
      t('mobile.accounts.sections.online'),
      onlineAccounts,
      expandedSectionIds.has('online'),
      displayCurrencyCode,
      rates,
    ),
    buildSection(
      'investment',
      t('mobile.accounts.sections.investment'),
      investmentAccounts,
      expandedSectionIds.has('investment'),
      displayCurrencyCode,
      rates,
    ),
    buildSection(
      'stored',
      t('mobile.accounts.sections.stored'),
      storedAccounts,
      expandedSectionIds.has('stored'),
      displayCurrencyCode,
      rates,
    ),
  ];
}

// buildSection receives section parts and returns a normalized account section.
function buildSection(
  id: string,
  label: string,
  accounts: Account[],
  expanded: boolean,
  displayCurrencyCode: string,
  rates: Map<string, number>,
): AccountSection {
  return {
    id,
    label,
    count: accounts.length,
    totalCents: accounts.reduce(
      (sum, account) => sum + accountDisplayBalanceCents(account, displayCurrencyCode, rates),
      0,
    ),
    expanded,
    accounts,
  };
}

// accountDisplayBalanceCents receives an account and returns its balance in the profile display currency.
function accountDisplayBalanceCents(account: Account, displayCurrencyCode: string, rates: Map<string, number>): number {
  return convertCurrencyAmountCents(account.openingBalanceCents, account.currency, displayCurrencyCode, rates) ?? 0;
}

// parseMajorAmountToCents receives a user-entered decimal amount and returns cents for API payloads.
function parseMajorAmountToCents(value: string): number {
  const normalized = value.trim();
  if (!normalized) {
    return 0;
  }

  const parsed = Number(normalized);
  if (!Number.isFinite(parsed)) {
    return 0;
  }

  return Math.round(parsed * 100);
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
  return (
    text.includes('online') ||
    text.includes('wallet') ||
    text.includes('paypal') ||
    text.includes('payment') ||
    text.includes('platform')
  );
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
