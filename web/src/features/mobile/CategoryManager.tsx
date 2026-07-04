import { Archive, RotateCcw, Save } from 'lucide-react';
import { type FormEvent, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { type Category, type CategoryCreateInput, type CategoryUpdateInput } from '../../lib/api/ledger';
import './category-manager.css';

type CategoryManagerProps = {
  canManageCategories: boolean;
  categories: Category[];
  defaultDirection?: string;
  isBusy: boolean;
  onCreateCategory: (input: CategoryCreateInput) => Promise<void>;
  onUpdateCategory: (categoryId: string, input: CategoryUpdateInput) => Promise<void>;
};

type CategoryDraft = {
  archived: boolean;
  direction: string;
  name: string;
};

// CategoryManager receives category data and returns create, rename, direction, and archive controls.
export function CategoryManager({ canManageCategories, categories, defaultDirection = 'expense', isBusy, onCreateCategory, onUpdateCategory }: CategoryManagerProps) {
  const { t } = useTranslation();
  const [name, setName] = useState('');
  const [direction, setDirection] = useState(defaultDirection);
  const sortedCategories = [...categories].sort((left, right) => Number(left.archived) - Number(right.archived) || left.name.localeCompare(right.name));

  // handleCreate receives a form event and creates a category for the selected book.
  async function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const normalizedName = name.trim();
    if (!normalizedName || !canManageCategories) {
      return;
    }

    await onCreateCategory({ name: normalizedName, direction });
    setName('');
  }

  return (
    <section className="categoryManager" aria-label={t('mobile.categories.title')}>
      <div className="sectionTitle">
        <h2>{t('mobile.categories.title')}</h2>
      </div>
      <form className="categoryCreateForm" aria-label={t('mobile.categories.create')} onSubmit={handleCreate}>
        <label>
          <span>{t('mobile.categories.name')}</span>
          <input
            aria-label={t('mobile.categories.name')}
            maxLength={80}
            placeholder={t('mobile.categories.namePlaceholder')}
            value={name}
            disabled={!canManageCategories}
            onChange={(event) => setName(event.target.value)}
          />
        </label>
        <label>
          <span>{t('mobile.categories.direction')}</span>
          <select aria-label={t('mobile.categories.direction')} value={direction} disabled={!canManageCategories} onChange={(event) => setDirection(event.target.value)}>
            <option value="expense">{t('mobile.record.types.expense')}</option>
            <option value="income">{t('mobile.record.types.income')}</option>
          </select>
        </label>
        <button type="submit" disabled={!canManageCategories || isBusy || !name.trim()}>{t('mobile.categories.create')}</button>
      </form>
      {sortedCategories.length ? (
        <ul className="categoryEditList">
          {sortedCategories.map((category) => (
            <CategoryEditor
              key={category.id}
              canManageCategories={canManageCategories}
              category={category}
              isBusy={isBusy}
              onUpdateCategory={onUpdateCategory}
            />
          ))}
        </ul>
      ) : (
        <p className="emptyState">{t('mobile.categories.empty')}</p>
      )}
    </section>
  );
}

// CategoryEditor receives one category and returns editable fields for that category.
function CategoryEditor({
  canManageCategories,
  category,
  isBusy,
  onUpdateCategory,
}: {
  canManageCategories: boolean;
  category: Category;
  isBusy: boolean;
  onUpdateCategory: (categoryId: string, input: CategoryUpdateInput) => Promise<void>;
}) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState<CategoryDraft>(() => buildCategoryDraft(category));
  const hasChanges = draft.name.trim() !== category.name || draft.direction !== category.direction || draft.archived !== category.archived;
  const canSave = canManageCategories && Boolean(draft.name.trim()) && hasChanges && !isBusy;

  // handleSave receives no parameters and patches the current category draft.
  async function handleSave() {
    if (!canSave) {
      return;
    }

    await onUpdateCategory(category.id, {
      archived: draft.archived,
      direction: draft.direction,
      name: draft.name.trim(),
    });
  }

  return (
    <li className={draft.archived ? 'categoryArchived' : ''}>
      <label>
        <span>{t('mobile.categories.nameFor', { name: category.name })}</span>
        <input
          aria-label={t('mobile.categories.nameFor', { name: category.name })}
          disabled={!canManageCategories}
          maxLength={80}
          value={draft.name}
          onChange={(event) => setDraft((current) => ({ ...current, name: event.target.value }))}
        />
      </label>
      <label>
        <span>{t('mobile.categories.directionFor', { name: category.name })}</span>
        <select
          aria-label={t('mobile.categories.directionFor', { name: category.name })}
          disabled={!canManageCategories}
          value={draft.direction}
          onChange={(event) => setDraft((current) => ({ ...current, direction: event.target.value }))}
        >
          <option value="expense">{t('mobile.record.types.expense')}</option>
          <option value="income">{t('mobile.record.types.income')}</option>
        </select>
      </label>
      <div className="categoryEditorActions">
        <button type="button" className="mobileSecondaryButton" disabled={!canManageCategories || isBusy} onClick={() => setDraft((current) => ({ ...current, archived: !current.archived }))}>
          {draft.archived ? <RotateCcw size={16} /> : <Archive size={16} />}
          {draft.archived ? t('mobile.categories.restore') : t('mobile.categories.archive')}
        </button>
        <button type="button" className="mobilePrimaryButton" disabled={!canSave} onClick={handleSave}>
          <Save size={16} />
          {t('mobile.categories.save')}
        </button>
      </div>
    </li>
  );
}

// buildCategoryDraft receives a category and returns local editable state.
function buildCategoryDraft(category: Category): CategoryDraft {
  return {
    archived: category.archived,
    direction: category.direction,
    name: category.name,
  };
}
