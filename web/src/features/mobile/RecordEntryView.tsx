import { Camera, ChevronDown, List, Search, X } from 'lucide-react';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  type Account,
  type BookListItem,
  type Category,
  type CategoryCreateInput,
  type CategoryUpdateInput,
  type Entry,
} from '@/lib/api/ledger';
import { formatMoney, supportedCurrencies } from '@/lib/money';
import { CategoryManager } from './CategoryManager';
import './record-entry.css';
import './record-category-sheet.css';

import {
  amountToCents,
  appendAmountKey,
  buildCategoryGroups,
  buildCategoryPreferences,
  buildCategoryShortcuts,
  calculateExpression,
  categoryIcon,
  categoryParentIds,
  filterCategories,
  formatAmountInput,
  keyAriaLabel,
  keyLabel,
  toDateTimeLocalValue,
  toSafeISOString,
  type RecordType,
} from './recordEntryUtils';

type CategoryDirection = CategoryCreateInput['direction'];

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
  const [member, setMember] = useState(members[0] ?? 'Family');
  const [currency, setCurrency] = useState(selectedBookCurrency || 'USD');
  const [amountExpression, setAmountExpression] = useState('0');
  const [note, setNote] = useState('');
  const [occurredAt, setOccurredAt] = useState(() => toDateTimeLocalValue(new Date()));
  const [isCategorySheetOpen, setIsCategorySheetOpen] = useState(false);
  const hasUserSelectedCategory = useRef(false);

  const visibleCategories = useMemo(() => filterCategories(categories, recordType), [categories, recordType]);
  const parentCategoryIds = useMemo(() => categoryParentIds(categories), [categories]);
  const selectableCategories = useMemo(
    () => visibleCategories.filter((category) => !parentCategoryIds.has(category.id)),
    [parentCategoryIds, visibleCategories],
  );
  const shortcuts = useMemo(
    () => buildCategoryShortcuts(selectableCategories, recentEntries),
    [recentEntries, selectableCategories],
  );
  const categoryPrefs = useMemo(() => buildCategoryPreferences(recentEntries), [recentEntries]);
  const selectedCategory =
    selectableCategories.find((category) => category.id === selectedCategoryId) ?? shortcuts[0]?.category;
  const selectedAccount = accounts.find((account) => account.id === accountId) ?? accounts[0];
  const destinationAccount = accounts.find(
    (account) => account.id === destinationAccountId && account.id !== selectedAccount?.id,
  );
  const amountCents = amountToCents(calculateExpression(amountExpression));
  const isMultiCurrency = supportedCurrencies.length > 1 || rates.size > 0;
  const canSave =
    Boolean(selectedAccount) && amountCents > 0 && (recordType !== 'transfer' || Boolean(destinationAccount));
  const categoryCreateDirection: CategoryDirection =
    recordType === 'income' || recordType === 'borrow' ? 'income' : 'expense';

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
    if (selectedCategoryId || !selectedCategory || hasUserSelectedCategory.current) {
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
    hasUserSelectedCategory.current = true;
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

  // handleSheetCategorySelect receives a category from the expanded sheet and returns to the compact composer.
  function handleSheetCategorySelect(category: Category) {
    applyCategoryPreference(category);
    setIsCategorySheetOpen(false);
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
          <button type="button" className="categoryAllShortcut" onClick={() => setIsCategorySheetOpen(true)}>
            <span>
              <List size={20} />
            </span>
            <b>{t('mobile.record.allCategories')}</b>
          </button>
        </div>

        <label className="noteLine">
          <input
            aria-label={t('common.note')}
            value={note}
            placeholder={t('mobile.record.notePlaceholder')}
            onChange={(event) => setNote(event.target.value)}
          />
          <Camera size={24} />
        </label>

        <div className="recordFieldChips" role="group" aria-label={t('mobile.record.fields')}>
          <label>
            <span>{t('mobile.record.time')}</span>
            <input
              aria-label={t('mobile.record.time')}
              type="datetime-local"
              value={occurredAt}
              onChange={(event) => setOccurredAt(event.target.value)}
            />
          </label>
          <label>
            <span>{t('mobile.record.account')}</span>
            <select
              aria-label={t('mobile.record.account')}
              value={selectedAccount?.id ?? ''}
              onChange={(event) => setAccountId(event.target.value)}
            >
              {accounts.map((account) => (
                <option key={account.id} value={account.id}>
                  {account.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            <span>{t('mobile.record.book')}</span>
            <select
              aria-label={t('mobile.record.book')}
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
                <option>{t('common.noBookYet')}</option>
              )}
            </select>
          </label>
          <label>
            <span>{t('mobile.record.member')}</span>
            <select
              aria-label={t('mobile.record.member')}
              value={member}
              onChange={(event) => setMember(event.target.value)}
            >
              {members.map((item) => (
                <option key={item} value={item}>
                  {item}
                </option>
              ))}
            </select>
          </label>
          {isMultiCurrency ? (
            <label>
              <span>{t('mobile.record.currency')}</span>
              <select
                aria-label={t('mobile.record.currency')}
                value={currency}
                onChange={(event) => setCurrency(event.target.value)}
              >
                {supportedCurrencies.map((item) => (
                  <option key={item} value={item}>
                    {item}
                  </option>
                ))}
              </select>
            </label>
          ) : null}
          {recordType === 'transfer' ? (
            <label>
              <span>{t('mobile.record.destination')}</span>
              <select
                aria-label={t('mobile.record.destination')}
                value={destinationAccount?.id ?? ''}
                onChange={(event) => setDestinationAccountId(event.target.value)}
              >
                {accounts
                  .filter((account) => account.id !== selectedAccount?.id)
                  .map((account) => (
                    <option key={account.id} value={account.id}>
                      {account.name}
                    </option>
                  ))}
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

      {isCategorySheetOpen ? (
        <CategorySheet
          canManageCategories={canManageCategories}
          categories={selectableCategories}
          defaultDirection={categoryCreateDirection}
          isBusy={isBusy}
          selectedCategoryId={selectedCategory?.id ?? ''}
          sourceCategories={categories}
          onClose={() => setIsCategorySheetOpen(false)}
          onCreateCategory={onCreateCategory}
          onSelectCategory={handleSheetCategorySelect}
          onUpdateCategory={onUpdateCategory}
        />
      ) : null}

      <div className="recordAside">
        <CategoryManager
          key={`aside-${categoryCreateDirection}`}
          canManageCategories={canManageCategories}
          categories={categories}
          defaultDirection={categoryCreateDirection}
          isBusy={isBusy}
          onCreateCategory={onCreateCategory}
          onUpdateCategory={onUpdateCategory}
        />
      </div>
    </section>
  );
}

// CategorySheet receives category state and returns a grouped full picker with management controls.
function CategorySheet({
  canManageCategories,
  categories,
  defaultDirection,
  isBusy,
  onClose,
  onCreateCategory,
  onSelectCategory,
  onUpdateCategory,
  selectedCategoryId,
  sourceCategories,
}: {
  canManageCategories: boolean;
  categories: Category[];
  defaultDirection: CategoryDirection;
  isBusy: boolean;
  onClose: () => void;
  onCreateCategory: (input: CategoryCreateInput) => Promise<void>;
  onSelectCategory: (category: Category) => void;
  onUpdateCategory: (categoryId: string, input: CategoryUpdateInput) => Promise<void>;
  selectedCategoryId: string;
  sourceCategories: Category[];
}) {
  const { t } = useTranslation();
  const [query, setQuery] = useState('');
  const normalizedQuery = query.trim().toLowerCase();
  const visibleCategories = normalizedQuery
    ? categories.filter((category) => category.name.toLowerCase().includes(normalizedQuery))
    : categories;
  const groups = buildCategoryGroups(visibleCategories, sourceCategories, t);

  return (
    <div className="categorySheetOverlay" role="presentation">
      <section className="categorySheet" aria-label={t('mobile.record.categorySheetTitle')}>
        <header className="categorySheetHeader">
          <button type="button" aria-label={t('mobile.record.closeCategories')} onClick={onClose}>
            <X size={22} aria-hidden="true" />
          </button>
          <h2>{t('mobile.record.categorySheetTitle')}</h2>
          <span aria-hidden="true" />
        </header>

        <label className="categorySheetSearch">
          <Search size={18} aria-hidden="true" />
          <input
            aria-label={t('mobile.record.searchCategories')}
            placeholder={t('mobile.record.searchCategoriesPlaceholder')}
            value={query}
            onChange={(event) => setQuery(event.target.value)}
          />
        </label>

        <div className="categorySheetBody">
          {groups.length ? (
            groups.map((group) => (
              <details className="categorySheetGroup" key={group.id} open>
                <summary>
                  <span>{categoryIcon(group.title)}</span>
                  <b>{group.title}</b>
                  <ChevronDown className="categorySheetChevron" size={18} aria-hidden="true" />
                </summary>
                <div className="categorySheetList">
                  {group.categories.map((category) => (
                    <button
                      key={category.id}
                      type="button"
                      className={
                        category.id === selectedCategoryId
                          ? 'categorySheetItem categorySheetItemActive'
                          : 'categorySheetItem'
                      }
                      onClick={() => onSelectCategory(category)}
                    >
                      <span>{categoryIcon(category.name)}</span>
                      <b>{category.name}</b>
                    </button>
                  ))}
                </div>
              </details>
            ))
          ) : (
            <p className="categorySheetEmpty">{t('mobile.categories.empty')}</p>
          )}

          <details className="categorySheetManage">
            <summary>{t('mobile.record.manageCategories')}</summary>
            <CategoryManager
              key={`sheet-${defaultDirection}`}
              canManageCategories={canManageCategories}
              categories={sourceCategories}
              defaultDirection={defaultDirection}
              isBusy={isBusy}
              onCreateCategory={onCreateCategory}
              onUpdateCategory={onUpdateCategory}
            />
          </details>
        </div>
      </section>
    </div>
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
          <button
            key={key}
            type="button"
            aria-label={keyAriaLabel(key, t)}
            className={key === 'calculator' ? 'calculatorToggleKey' : ''}
            onClick={() => handlePadPress(key)}
          >
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
