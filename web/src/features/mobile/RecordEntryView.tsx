import {
  ArrowLeftRight,
  Banknote,
  Camera,
  Car,
  Calculator,
  Flame,
  Fuel,
  Gift,
  Home,
  Package,
  ParkingCircle,
  PiggyBank,
  Smartphone,
  Trash2,
  Utensils,
} from 'lucide-react';
import { type TFunction } from 'i18next';
import { type ReactNode, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { type Account, type BookListItem, type Category, type CategoryCreateInput, type CategoryUpdateInput, type Entry } from '../../lib/api/ledger';
import { formatMoney, supportedCurrencies } from '../../lib/money';
import { CategoryManager } from './CategoryManager';
import './record-entry.css';

type RecordType = 'income' | 'expense' | 'transfer' | 'borrow';

export type RecordEntryInput = {
  type: string;
  accountId: string;
  destinationAccountId?: string;
  categoryId?: string;
  categoryName?: string;
  amountCents: number;
  transactionCurrency: string;
  occurredAt: string;
  note?: string;
  member: string;
};

type RecordEntryViewProps = {
  accounts: Account[];
  books: BookListItem[];
  canManageCategories: boolean;
  categories: Category[];
  isBusy: boolean;
  onCreateCategory: (input: CategoryCreateInput) => Promise<void>;
  onCreateEntry: (input: RecordEntryInput) => Promise<void>;
  onUpdateCategory: (categoryId: string, input: CategoryUpdateInput) => Promise<void>;
  rates: Map<string, number>;
  recentEntries: Entry[];
  selectedBookCurrency: string;
  selectedBookId: string;
  setSelectedBookId: (value: string) => void;
};

type CategoryShortcut = {
  category: Category;
  count: number;
};

const recordTypes: Array<{ id: RecordType; entryType: string }> = [
  { id: 'income', entryType: 'income' },
  { id: 'expense', entryType: 'expense' },
  { id: 'transfer', entryType: 'transfer' },
  { id: 'borrow', entryType: 'borrow' },
];

const members = ['Family', 'Me'];

// RecordEntryView receives ledger context and returns the mobile entry composer.
export function RecordEntryView({
  accounts,
  books,
  canManageCategories,
  categories,
  isBusy,
  onCreateCategory,
  onCreateEntry,
  onUpdateCategory,
  rates,
  recentEntries,
  selectedBookCurrency,
  selectedBookId,
  setSelectedBookId,
}: RecordEntryViewProps) {
  const { t } = useTranslation();
  const [recordType, setRecordType] = useState<RecordType>('expense');
  const [selectedCategoryId, setSelectedCategoryId] = useState('');
  const [accountId, setAccountId] = useState('');
  const [destinationAccountId, setDestinationAccountId] = useState('');
  const [member, setMember] = useState(members[0]);
  const [currency, setCurrency] = useState(selectedBookCurrency || 'USD');
  const [amountExpression, setAmountExpression] = useState('0');
  const [note, setNote] = useState('');
  const [occurredAt, setOccurredAt] = useState(() => toDateTimeLocalValue(new Date()));

  const visibleCategories = useMemo(() => filterCategories(categories, recordType), [categories, recordType]);
  const shortcuts = useMemo(() => buildCategoryShortcuts(visibleCategories, recentEntries), [recentEntries, visibleCategories]);
  const categoryPrefs = useMemo(() => buildCategoryPreferences(recentEntries), [recentEntries]);
  const selectedCategory = visibleCategories.find((category) => category.id === selectedCategoryId) ?? shortcuts[0]?.category;
  const selectedAccount = accounts.find((account) => account.id === accountId) ?? accounts[0];
  const destinationAccount = accounts.find((account) => account.id === destinationAccountId && account.id !== selectedAccount?.id);
  const amountCents = amountToCents(calculateExpression(amountExpression));
  const isMultiCurrency = supportedCurrencies.length > 1 || rates.size > 0;
  const canSave = Boolean(selectedAccount) && amountCents > 0 && (recordType !== 'transfer' || Boolean(destinationAccount));

  useEffect(() => {
    if (!accountId && accounts[0]) {
      setAccountId(accounts[0].id);
    }
    if (!destinationAccountId) {
      setDestinationAccountId(accounts.find((account) => account.id !== accounts[0]?.id)?.id ?? '');
    }
  }, [accountId, accounts, destinationAccountId]);

  useEffect(() => {
    setCurrency((current) => current || selectedBookCurrency || 'USD');
  }, [selectedBookCurrency]);

  useEffect(() => {
    if (selectedCategoryId || !selectedCategory) {
      return;
    }
    setSelectedCategoryId(selectedCategory.id);
    const preference = categoryPrefs.get(selectedCategory.id);
    if (preference?.accountId) {
      setAccountId(preference.accountId);
    }
    if (preference?.currency) {
      setCurrency(preference.currency);
    }
    if (preference?.member) {
      setMember(preference.member);
    }
  }, [categoryPrefs, selectedCategory, selectedCategoryId]);

  // applyCategoryPreference receives a category and applies its most recent account, member, and currency choices.
  function applyCategoryPreference(category: Category) {
    setSelectedCategoryId(category.id);
    const preference = categoryPrefs.get(category.id);
    if (preference?.accountId) {
      setAccountId(preference.accountId);
    }
    if (preference?.currency) {
      setCurrency(preference.currency);
    }
    if (preference?.member) {
      setMember(preference.member);
    }
  }

  // handleKeyPress receives one calculator key and updates the amount expression safely.
  function handleKeyPress(key: string) {
    if (key === 'backspace') {
      setAmountExpression((current) => (current.length > 1 ? current.slice(0, -1) : '0'));
      return;
    }
    if (key === 'clear') {
      setAmountExpression('0');
      return;
    }
    if (key === '=') {
      setAmountExpression(formatAmountInput(calculateExpression(amountExpression)));
      return;
    }
    setAmountExpression((current) => appendAmountKey(current, key));
  }

  // handleSave receives no parameters and posts the current entry draft.
  async function handleSave() {
    if (!canSave || !selectedAccount) {
      return;
    }
    await onCreateEntry({
      type: recordTypes.find((type) => type.id === recordType)?.entryType ?? 'expense',
      accountId: selectedAccount.id,
      destinationAccountId: recordType === 'transfer' ? destinationAccount?.id : undefined,
      categoryId: selectedCategory?.id,
      categoryName: selectedCategory?.name ?? t('mobile.record.categoryFallback'),
      amountCents,
      transactionCurrency: currency,
      occurredAt: toSafeISOString(occurredAt),
      note,
      member,
    });
    setAmountExpression('0');
    setNote('');
  }

  return (
    <section className="tabPanel recordEntryPanel" aria-label={t('mobile.record.title')}>
      <div className="recordComposer">
        <div className="recordHero" aria-hidden="true" />
        <div className="recordTypeTabs" role="tablist" aria-label={t('mobile.record.typeLabel')}>
          {recordTypes.map((type) => (
            <button
              key={type.id}
              type="button"
              role="tab"
              aria-selected={recordType === type.id}
              className={recordType === type.id ? 'recordTypeActive' : ''}
              onClick={() => setRecordType(type.id)}
            >
              {t(`mobile.record.types.${type.id}`)}
            </button>
          ))}
        </div>

        <section className="selectedCategoryCard" aria-label={t('mobile.record.selectedCategory')}>
          <span>{categoryIcon(selectedCategory?.name ?? recordType)}</span>
          <strong>{selectedCategory?.name ?? t('mobile.record.categoryFallback')}</strong>
          <b>{formatMoney(amountCents, currency)}</b>
        </section>

        <div className="categoryShortcutGrid" role="group" aria-label={t('mobile.record.categoryShortcuts')}>
          {shortcuts.map((shortcut) => (
            <button
              key={shortcut.category.id}
              type="button"
              className={selectedCategory?.id === shortcut.category.id ? 'categoryShortcutActive' : ''}
              onClick={() => applyCategoryPreference(shortcut.category)}
            >
              <span>{categoryIcon(shortcut.category.name)}</span>
              <b>{shortcut.category.name}</b>
            </button>
          ))}
        </div>

        <label className="noteLine">
          <input aria-label={t('common.note')} value={note} placeholder={t('mobile.record.notePlaceholder')} onChange={(event) => setNote(event.target.value)} />
          <Camera size={24} />
        </label>

        <div className="recordFieldChips" role="group" aria-label={t('mobile.record.fields')}>
          <label>
            <span>{t('mobile.record.time')}</span>
            <input aria-label={t('mobile.record.time')} type="datetime-local" value={occurredAt} onChange={(event) => setOccurredAt(event.target.value)} />
          </label>
          <label>
            <span>{t('mobile.record.account')}</span>
            <select aria-label={t('mobile.record.account')} value={selectedAccount?.id ?? ''} onChange={(event) => setAccountId(event.target.value)}>
              {accounts.map((account) => <option key={account.id} value={account.id}>{account.name}</option>)}
            </select>
          </label>
          <label>
            <span>{t('mobile.record.book')}</span>
            <select aria-label={t('mobile.record.book')} value={selectedBookId} onChange={(event) => setSelectedBookId(event.target.value)} disabled={!books.length}>
              {books.length ? books.map((book) => <option key={book.id} value={book.id}>{book.name}</option>) : <option>{t('common.noBookYet')}</option>}
            </select>
          </label>
          <label>
            <span>{t('mobile.record.member')}</span>
            <select aria-label={t('mobile.record.member')} value={member} onChange={(event) => setMember(event.target.value)}>
              {members.map((item) => <option key={item} value={item}>{item}</option>)}
            </select>
          </label>
          {isMultiCurrency ? (
            <label>
              <span>{t('mobile.record.currency')}</span>
              <select aria-label={t('mobile.record.currency')} value={currency} onChange={(event) => setCurrency(event.target.value)}>
                {supportedCurrencies.map((item) => <option key={item} value={item}>{item}</option>)}
              </select>
            </label>
          ) : null}
          {recordType === 'transfer' ? (
            <label>
              <span>{t('mobile.record.destination')}</span>
              <select aria-label={t('mobile.record.destination')} value={destinationAccount?.id ?? ''} onChange={(event) => setDestinationAccountId(event.target.value)}>
                {accounts.filter((account) => account.id !== selectedAccount?.id).map((account) => <option key={account.id} value={account.id}>{account.name}</option>)}
              </select>
            </label>
          ) : null}
        </div>

        <CalculatorPad
          amountExpression={amountExpression}
          canSave={canSave && !isBusy}
          onKeyPress={handleKeyPress}
          onSave={handleSave}
        />
      </div>

      <div className="recordAside">
        <CategoryManager
          canManageCategories={canManageCategories}
          categories={categories}
          isBusy={isBusy}
          onCreateCategory={onCreateCategory}
          onUpdateCategory={onUpdateCategory}
        />
      </div>
    </section>
  );
}

// CalculatorPad receives calculator state and returns the fixed-format keypad.
function CalculatorPad({
  amountExpression,
  canSave,
  onKeyPress,
  onSave,
}: {
  amountExpression: string;
  canSave: boolean;
  onKeyPress: (key: string) => void;
  onSave: () => void;
}) {
  const { t } = useTranslation();
  const [isCalculatorOpen, setIsCalculatorOpen] = useState(false);
  const keys = isCalculatorOpen
    ? ['1', '2', '3', 'backspace', '4', '5', '6', '+', '7', '8', '9', '-', '0', '.', '*', '/', 'clear', '=']
    : ['1', '2', '3', 'backspace', '4', '5', '6', 'calculator', '7', '8', '9', 'clear', '0', '.'];

  function handlePadPress(key: string) {
    if (key === 'calculator') {
      setIsCalculatorOpen(true);
      return;
    }
    onKeyPress(key);
  }

  return (
    <section className="calculatorPad" aria-label={t('mobile.record.calculator')}>
      <div className="calculatorDisplay">
        <output aria-label={t('common.amount')}>{amountExpression}</output>
        {isCalculatorOpen ? (
          <button
            type="button"
            aria-label={t('mobile.record.useCalculatorResult')}
            onClick={() => {
              onKeyPress('=');
              setIsCalculatorOpen(false);
            }}
          >
            {t('mobile.record.useResult')}
          </button>
        ) : null}
      </div>
      <div className={`calculatorGrid ${isCalculatorOpen ? 'calculatorGridOpen' : ''}`}>
        {keys.map((key) => (
          <button key={key} type="button" aria-label={keyAriaLabel(key, t)} className={key === 'calculator' ? 'calculatorToggleKey' : ''} onClick={() => handlePadPress(key)}>
            {keyLabel(key)}
          </button>
        ))}
        <button className="saveKey" type="button" disabled={!canSave} onClick={onSave}>
          {t('mobile.record.save')}
        </button>
      </div>
    </section>
  );
}

// amountToCents receives a decimal amount and returns whole cents.
function amountToCents(value: number): number {
  if (!Number.isFinite(value) || value <= 0) {
    return 0;
  }

  return Math.round(value * 100);
}

// appendAmountKey receives the current expression and key and returns the next safe expression.
function appendAmountKey(current: string, key: string): string {
  if (!/^[0-9.+\-*/]$/.test(key)) {
    return current;
  }
  if (current === '0' && /[0-9.]/.test(key)) {
    return key === '.' ? '0.' : key;
  }
  if (/[+\-*/.]$/.test(current) && /[+\-*/.]/.test(key)) {
    return `${current.slice(0, -1)}${key}`;
  }

  return `${current}${key}`.slice(0, 24);
}

// buildCategoryPreferences receives recent entries and returns last-used choices by category.
function buildCategoryPreferences(entries: Entry[]): Map<string, { accountId?: string; currency?: string; member?: string }> {
  const preferences = new Map<string, { accountId?: string; currency?: string; member?: string }>();
  const sortedEntries = [...entries].sort((left, right) => new Date(right.occurredAt).getTime() - new Date(left.occurredAt).getTime());
  for (const entry of sortedEntries) {
    if (!entry.categoryId || preferences.has(entry.categoryId)) {
      continue;
    }
    preferences.set(entry.categoryId, {
      accountId: entry.accountId,
      currency: entry.transactionCurrency,
      member: 'Family',
    });
  }

  return preferences;
}

// buildCategoryShortcuts receives categories and entries and returns usage-ranked category shortcuts.
function buildCategoryShortcuts(categories: Category[], entries: Entry[]): CategoryShortcut[] {
  const counts = new Map<string, number>();
  for (const entry of entries) {
    if (entry.categoryId) {
      counts.set(entry.categoryId, (counts.get(entry.categoryId) ?? 0) + 1);
    }
  }

  return categories
    .filter((category) => !category.archived)
    .map((category) => ({ category, count: counts.get(category.id) ?? 0 }))
    .sort((left, right) => right.count - left.count || left.category.sortOrder - right.category.sortOrder || left.category.name.localeCompare(right.category.name))
    .slice(0, 16);
}

// calculateExpression receives a calculator expression and returns the evaluated value without using eval.
function calculateExpression(expression: string): number {
  const tokens = expression.match(/[+\-*/]|\d+(?:\.\d+)?|\.\d+/g) ?? ['0'];
  const values: number[] = [];
  const operators: string[] = [];
  for (const token of tokens) {
    if (/^[+\-*/]$/.test(token)) {
      while (operators.length && precedence(operators[operators.length - 1]) >= precedence(token)) {
        applyTopOperator(values, operators);
      }
      operators.push(token);
      continue;
    }
    values.push(Number(token));
  }
  while (operators.length) {
    applyTopOperator(values, operators);
  }

  return values[0] ?? 0;
}

// applyTopOperator receives value and operator stacks and reduces the top arithmetic operation.
function applyTopOperator(values: number[], operators: string[]) {
  const operator = operators.pop();
  const right = values.pop() ?? 0;
  const left = values.pop() ?? 0;
  switch (operator) {
    case '+':
      values.push(left + right);
      return;
    case '-':
      values.push(left - right);
      return;
    case '*':
      values.push(left * right);
      return;
    case '/':
      values.push(right === 0 ? left : left / right);
      return;
    default:
      values.push(right);
  }
}

// categoryIcon receives category text and returns a matching shortcut icon.
function categoryIcon(value: string): ReactNode {
  const normalized = value.toLowerCase();
  if (normalized.includes('income') || normalized.includes('salary')) {
    return <Banknote size={22} />;
  }
  if (normalized.includes('transfer')) {
    return <ArrowLeftRight size={22} />;
  }
  if (normalized.includes('borrow') || normalized.includes('loan')) {
    return <PiggyBank size={22} />;
  }
  if (normalized.includes('lunch') || normalized.includes('dining') || normalized.includes('dinner')) {
    return <Utensils size={22} />;
  }
  if (normalized.includes('fuel') || normalized.includes('gas')) {
    return <Fuel size={22} />;
  }
  if (normalized.includes('utility') || normalized.includes('water')) {
    return <Flame size={22} />;
  }
  if (normalized.includes('parking')) {
    return <ParkingCircle size={22} />;
  }
  if (normalized.includes('phone')) {
    return <Smartphone size={22} />;
  }
  if (normalized.includes('rent') || normalized.includes('home')) {
    return <Home size={22} />;
  }
  if (normalized.includes('toy')) {
    return <Package size={22} />;
  }
  if (normalized.includes('gift')) {
    return <Gift size={22} />;
  }
  if (normalized.includes('transport') || normalized.includes('car')) {
    return <Car size={22} />;
  }

  return <Package size={22} />;
}

// filterCategories receives all categories and the active type and returns compatible shortcuts.
function filterCategories(categories: Category[], recordType: RecordType): Category[] {
  if (recordType === 'income' || recordType === 'borrow') {
    return categories.filter((category) => category.direction === 'income' && !category.archived);
  }
  if (recordType === 'expense') {
    return categories.filter((category) => category.direction === 'expense' && !category.archived);
  }

  return categories.filter((category) => !category.archived);
}

// formatAmountInput receives a calculated value and returns compact calculator text.
function formatAmountInput(value: number): string {
  if (!Number.isFinite(value)) {
    return '0';
  }

  return String(Math.max(0, Math.round(value * 100) / 100));
}

// keyLabel receives an internal calculator key and returns visible button text.
function keyLabel(key: string): ReactNode {
  if (key === 'backspace') {
    return <Trash2 size={18} />;
  }
  if (key === 'calculator') {
    return <Calculator size={19} />;
  }
  if (key === 'clear') {
    return 'C';
  }

  return key;
}

// keyAriaLabel receives an internal calculator key and returns an accessible button label.
function keyAriaLabel(key: string, t: TFunction): string | undefined {
  switch (key) {
    case 'backspace':
      return t('mobile.record.backspace');
    case 'calculator':
      return t('mobile.record.openCalculator');
    case 'clear':
      return t('mobile.record.clearAmount');
    case '=':
      return t('mobile.record.applyCalculation');
    default:
      return undefined;
  }
}

// precedence receives an operator and returns its arithmetic precedence.
function precedence(operator: string): number {
  return operator === '*' || operator === '/' ? 2 : 1;
}

// toDateTimeLocalValue receives a date and returns a datetime-local input value.
function toDateTimeLocalValue(value: Date): string {
  // Use local wall-clock components. A datetime-local input is interpreted as
  // local time, and toSafeISOString parses it back with new Date(value) as local
  // time, so building the default from getUTC* would shift occurredAt by the
  // timezone offset (e.g. wrong day/month for users east of UTC).
  const year = value.getFullYear();
  const month = String(value.getMonth() + 1).padStart(2, '0');
  const day = String(value.getDate()).padStart(2, '0');
  const hour = String(value.getHours()).padStart(2, '0');
  const minute = String(value.getMinutes()).padStart(2, '0');
  return `${year}-${month}-${day}T${hour}:${minute}`;
}

// toSafeISOString receives a local datetime value and returns a safe ISO timestamp.
function toSafeISOString(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return new Date().toISOString();
  }

  return date.toISOString();
}
