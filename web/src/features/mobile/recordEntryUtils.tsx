import {
  ArrowLeftRight,
  Banknote,
  Calculator,
  Car,
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
import { type ReactNode } from 'react';
import { type Category, type Entry } from '@/lib/api/ledger';

export type RecordType = 'income' | 'expense' | 'transfer' | 'borrow';

type CategoryShortcut = {
  category: Category;
  count: number;
};

type CategoryGroup = {
  id: string;
  title: string;
  categories: Category[];
};

// amountToCents receives a decimal amount and returns whole cents.
export function amountToCents(value: number): number {
  if (!Number.isFinite(value) || value <= 0) {
    return 0;
  }

  return Math.round(value * 100);
}

// appendAmountKey receives the current expression and key and returns the next safe expression.
export function appendAmountKey(current: string, key: string): string {
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
export function buildCategoryPreferences(
  entries: Entry[],
): Map<string, { accountId?: string; currency?: string; member?: string }> {
  const preferences = new Map<string, { accountId?: string; currency?: string; member?: string }>();
  const sortedEntries = [...entries].sort(
    (left, right) => new Date(right.occurredAt).getTime() - new Date(left.occurredAt).getTime(),
  );
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

// categoryParentIds receives all categories and returns ids currently used to group child categories.
export function categoryParentIds(categories: Category[]): Set<string> {
  const ids = new Set<string>();
  for (const category of categories) {
    if (!category.archived && category.parentId) {
      ids.add(category.parentId);
    }
  }

  return ids;
}

// buildCategoryShortcuts receives categories and entries and returns usage-ranked category shortcuts.
export function buildCategoryShortcuts(categories: Category[], entries: Entry[]): CategoryShortcut[] {
  const counts = new Map<string, number>();
  for (const entry of entries) {
    if (entry.categoryId) {
      counts.set(entry.categoryId, (counts.get(entry.categoryId) ?? 0) + 1);
    }
  }

  return categories
    .filter((category) => !category.archived)
    .map((category) => ({ category, count: counts.get(category.id) ?? 0 }))
    .sort(
      (left, right) =>
        right.count - left.count ||
        left.category.sortOrder - right.category.sortOrder ||
        left.category.name.localeCompare(right.category.name),
    )
    .slice(0, 7);
}

// buildCategoryGroups receives compatible categories and groups them by parent category or direction.
export function buildCategoryGroups(
  categories: Category[],
  sourceCategories: Category[],
  t: TFunction,
): CategoryGroup[] {
  const parentById = new Map(sourceCategories.map((category) => [category.id, category]));
  const groups = new Map<string, CategoryGroup>();

  for (const category of [...categories].sort(
    (left, right) => left.sortOrder - right.sortOrder || left.name.localeCompare(right.name),
  )) {
    const parent = category.parentId ? parentById.get(category.parentId) : undefined;
    const groupId = parent ? `parent-${parent.id}` : `direction-${category.direction}`;
    const title = parent?.name ?? categoryDirectionLabel(category.direction, t);
    const group: CategoryGroup = groups.get(groupId) ?? { id: groupId, title, categories: [] };
    group.categories.push(category);
    groups.set(groupId, group);
  }

  return [...groups.values()];
}

// categoryDirectionLabel receives a category direction and returns the localized group heading.
function categoryDirectionLabel(direction: string, t: TFunction): string {
  if (direction === 'income') {
    return t('mobile.record.types.income');
  }
  if (direction === 'expense') {
    return t('mobile.record.types.expense');
  }

  return t('mobile.record.uncategorized');
}

// calculateExpression receives a calculator expression and returns the evaluated value without using eval.
export function calculateExpression(expression: string): number {
  const tokens = expression.match(/[+\-*/]|\d+(?:\.\d+)?|\.\d+/g) ?? ['0'];
  const values: number[] = [];
  const operators: string[] = [];
  for (const token of tokens) {
    if (/^[+\-*/]$/.test(token)) {
      while (operators.length) {
        const topOperator = operators.at(-1);
        if (!topOperator || precedence(topOperator) < precedence(token)) {
          break;
        }
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
export function categoryIcon(value: string): ReactNode {
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
export function filterCategories(categories: Category[], recordType: RecordType): Category[] {
  if (recordType === 'income' || recordType === 'borrow') {
    return categories.filter((category) => category.direction === 'income' && !category.archived);
  }
  if (recordType === 'expense') {
    return categories.filter((category) => category.direction === 'expense' && !category.archived);
  }

  return categories.filter((category) => !category.archived);
}

// formatAmountInput receives a calculated value and returns compact calculator text.
export function formatAmountInput(value: number): string {
  if (!Number.isFinite(value)) {
    return '0';
  }

  return String(Math.max(0, Math.round(value * 100) / 100));
}

// keyLabel receives an internal calculator key and returns visible button text.
export function keyLabel(key: string): ReactNode {
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
export function keyAriaLabel(key: string, t: TFunction): string | undefined {
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
export function toDateTimeLocalValue(value: Date): string {
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
export function toSafeISOString(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return new Date().toISOString();
  }

  return date.toISOString();
}
