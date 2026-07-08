import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { beforeEach, describe, expect, it } from 'vitest';
import {
  fixtureCategory,
  installAppTestFetchMock,
  openRecordTab,
  renderApp,
  setCurrentCategories,
} from './test/appTestHarness';

describe('App', () => {
  beforeEach(installAppTestFetchMock);

  it('renders the mobile home with stable active bottom navigation', async () => {
    renderApp();

    expect(await screen.findByRole('region', { name: 'Home' })).toBeInTheDocument();
    expect(screen.getByRole('region', { name: 'Monthly spending budget' })).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole('region', { name: 'Transactions' })).toHaveTextContent('Lunch'));

    const nav = screen.getByRole('navigation', { name: 'Main navigation' });
    expect(within(nav).getAllByRole('button')).toHaveLength(5);
    expect(within(nav).getByRole('button', { name: 'Accounts' })).toBeInTheDocument();
    expect(within(nav).getByRole('button', { name: 'Record' })).toBeInTheDocument();
    expect(within(nav).getByRole('button', { name: 'Home' })).toHaveAttribute('aria-current', 'page');
    expect(within(nav).getByRole('button', { name: 'Reports' })).toBeInTheDocument();
    expect(within(nav).getByRole('button', { name: 'Me' })).toBeInTheDocument();
    expect(within(nav).queryByRole('button', { name: 'Import' })).not.toBeInTheDocument();

    fireEvent.click(within(nav).getByRole('button', { name: 'Record' }));
    expect(await screen.findByRole('region', { name: 'Record entry' })).toBeInTheDocument();
    expect(within(nav).getByRole('button', { name: 'Record' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('tab', { name: 'Income' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Expense' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tab', { name: 'Transfer' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Borrow/Lend' })).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole('region', { name: 'Selected category' })).toHaveTextContent('Dining'));
    expect(screen.getByRole('group', { name: 'Entry fields' })).toBeInTheDocument();

    fireEvent.click(within(nav).getByRole('button', { name: 'Home' }));
    expect(await screen.findByRole('region', { name: 'Home' })).toBeInTheDocument();
    expect(within(nav).getByRole('button', { name: 'Home' })).toHaveAttribute('aria-current', 'page');
    expect(fetch).toHaveBeenCalledWith('/api/v1/ledger/summary', { signal: expect.any(AbortSignal) });
  });

  it('uses the header book switcher and workspace menu actions', async () => {
    renderApp('/home');

    expect(await screen.findByRole('region', { name: 'Home' })).toBeInTheDocument();
    await waitFor(() => expect(screen.getByLabelText('Switch book')).toHaveValue('book-1'));

    fireEvent.click(screen.getByRole('button', { name: 'More options' }));
    expect(screen.getByLabelText('Select language')).toBeInTheDocument();
    expect(screen.getByText('Theme')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Dark' }));
    expect(screen.getByRole('main')).toHaveClass('mobileShellThemeDark');

    fireEvent.click(screen.getByRole('button', { name: 'Edit book' }));
    expect(await screen.findByRole('region', { name: 'Accounts' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Accounts' })).toBeInTheDocument();
    expect(screen.getByLabelText('Book name')).toHaveValue('Household');
  });

  it.each([
    ['/home', 'Home', 'Home'],
    ['/accounts', 'Accounts', 'Accounts'],
    ['/record', 'Record entry', 'Record'],
    ['/reports/category', 'Reports', 'Reports'],
    ['/imports', 'Import data', 'Me'],
    ['/me', 'Me', 'Me'],
  ])('opens %s directly', async (path, regionName, navName) => {
    renderApp(path);

    expect(await screen.findByRole('region', { name: regionName })).toBeInTheDocument();
    const nav = await screen.findByRole('navigation', { name: 'Main navigation' });
    expect(within(nav).getByRole('button', { name: navName })).toHaveAttribute('aria-current', 'page');
  });

  it('opens Home transaction rows into a bill detail page', async () => {
    renderApp('/home');

    const transactions = await screen.findByRole('region', { name: 'Transactions' });
    await waitFor(() => expect(transactions).toHaveTextContent('Lunch'));
    fireEvent.click(within(transactions).getByRole('button', { name: 'Open Lunch detail' }));

    expect(await screen.findByRole('region', { name: 'Bill detail for Lunch' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Expense' })).toBeInTheDocument();
    const billFacts = screen.getByRole('region', { name: 'Bill facts' });
    expect(billFacts).toHaveTextContent('Payment account');
    expect(billFacts).toHaveTextContent('Cash');
    // Book members load lazily on entry detail, so the creator name resolves asynchronously.
    await waitFor(() => expect(billFacts).toHaveTextContent('Person'));
    expect(screen.getByRole('region', { name: 'Source' })).toHaveTextContent('Manual entry');
    expect(screen.getByRole('button', { name: 'Home' })).toHaveAttribute('aria-current', 'page');

    fireEvent.click(screen.getByRole('button', { name: 'More options' }));
    fireEvent.click(screen.getByRole('button', { name: 'Edit entry' }));
    const editor = await screen.findByRole('form', { name: 'Edit Lunch' });
    fireEvent.change(within(editor).getByLabelText('Note for Lunch'), { target: { value: 'Dinner' } });
    fireEvent.click(within(editor).getByRole('button', { name: 'Save details' }));

    expect(await screen.findByText('Entry updated.')).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledWith('/api/v1/books/book-1/entries/entry-1', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"note":"Dinner"'),
    });

    fireEvent.click(screen.getByRole('button', { name: 'Back to home' }));
    expect(await screen.findByRole('region', { name: 'Home' })).toBeInTheDocument();
  });

  it('opens a bill detail URL directly even when the entry is outside the Home page', async () => {
    renderApp('/entries/entry-cny');

    expect(await screen.findByRole('region', { name: 'Bill detail for Converted lunch' })).toBeInTheDocument();
    expect(screen.getByText('-CN¥100.00')).toBeInTheDocument();
    expect(screen.getByRole('region', { name: 'Source' })).toHaveTextContent('Manual entry');
    expect(fetch).toHaveBeenCalledWith('/api/v1/books/book-1/entries?page=1&page_size=100');
  });

  it.each([
    ['/reports/trend', 'Trend'],
    ['/reports/category', 'Category'],
    ['/reports/subcategory', 'Subcategory'],
    ['/reports/member', 'Member'],
    ['/reports/account', 'Account'],
    ['/reports/merchant', 'Merchant'],
    ['/reports/tag', 'Tag'],
  ])('opens %s directly', async (path, tabName) => {
    renderApp(path);

    expect(await screen.findByRole('tabpanel', { name: tabName })).toBeInTheDocument();
  });

  it('shows accounts and can prepare starter account data', async () => {
    renderApp();

    const nav = await screen.findByRole('navigation', { name: 'Main navigation' });
    fireEvent.click(within(nav).getByRole('button', { name: 'Accounts' }));

    expect(await screen.findByRole('region', { name: 'Accounts' })).toBeInTheDocument();
    expect(within(nav).getByRole('button', { name: 'Accounts' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('heading', { name: 'Accounts' })).toBeInTheDocument();
    expect(screen.getByRole('region', { name: 'Net assets' })).toBeInTheDocument();
    expect(screen.getByText('Credit cards')).toBeInTheDocument();
    expect(screen.getByText('Savings and IOUs')).toBeInTheDocument();
    expect(screen.getByText('cash / USD')).toBeInTheDocument();
    const creditCardsButton = screen.getByRole('button', { name: /Credit cards/ });
    expect(creditCardsButton).toHaveTextContent('(4)');
    expect(screen.getByRole('button', { name: /Stored-value cards/ })).toHaveTextContent('(0)');
    expect(screen.queryByText('Visa Blue')).not.toBeInTheDocument();
    fireEvent.click(creditCardsButton);
    expect(await screen.findByText('Visa Blue')).toBeInTheDocument();
    expect(screen.getByText('Backup Card')).toBeInTheDocument();
    const membersPanel = await screen.findByRole('article', { name: 'Book members' });
    await waitFor(() => expect(membersPanel).toHaveTextContent('Person'));
    expect(membersPanel).toHaveTextContent('owner');
    expect(fetch).toHaveBeenCalledWith('/api/v1/books/book-1/members?page=1&page_size=50');

    fireEvent.change(screen.getByLabelText('Book name'), { target: { value: 'Household 2026' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save book' }));

    expect(await screen.findByText('Book updated.')).toBeInTheDocument();
    expect(await screen.findByLabelText('Book name')).toHaveValue('Household 2026');
    expect(fetch).toHaveBeenCalledWith('/api/v1/books/book-1', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"name":"Household 2026"'),
    });

    fireEvent.change(screen.getByLabelText('Account group name'), { target: { value: 'Daily Wallets' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save group' }));

    expect(await screen.findByText('Account group updated.')).toBeInTheDocument();
    expect(await screen.findByLabelText('Account group name')).toHaveValue('Daily Wallets');
    expect(fetch).toHaveBeenCalledWith('/api/v1/accounts/groups/group-1', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"name":"Daily Wallets"'),
    });

    const accountsRegion = screen.getByRole('region', { name: 'Accounts' });
    fireEvent.click(within(accountsRegion).getByRole('button', { name: 'Prepare account' }));
    expect(await screen.findByText('Account ready.')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Account name'), { target: { value: 'Travel wallet' } });
    fireEvent.change(screen.getByLabelText('Type'), { target: { value: 'credit_card' } });
    fireEvent.change(screen.getByLabelText('Opening balance'), { target: { value: '123.45' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create account' }));

    expect(await screen.findByText('Account created.')).toBeInTheDocument();
    expect(await screen.findByText('Travel wallet')).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledWith('/api/v1/accounts', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"name":"Travel wallet"'),
    });
    expect(fetch).toHaveBeenCalledWith('/api/v1/accounts', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"type":"credit_card"'),
    });
    expect(fetch).toHaveBeenCalledWith('/api/v1/accounts', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"openingBalanceCents":12345'),
    });
  });

  it('posts a quick ledger entry with a calculator result from the record tab', async () => {
    renderApp();

    expect(await openRecordTab()).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole('group', { name: 'Category shortcuts' })).toHaveTextContent('Dining'));
    fireEvent.click(screen.getByRole('button', { name: '2' }));
    fireEvent.click(screen.getByRole('button', { name: '4' }));
    fireEvent.click(screen.getByRole('button', { name: 'Open calculator' }));
    fireEvent.click(screen.getByRole('button', { name: '+' }));
    fireEvent.click(screen.getByRole('button', { name: '6' }));
    fireEvent.click(screen.getByRole('button', { name: 'Apply calculation' }));
    fireEvent.change(screen.getByPlaceholderText('Add a note...'), { target: { value: 'Team lunch' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    expect(await screen.findByText('Entry posted.')).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledWith('/api/v1/books/book-1/entries', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"note":"Team lunch"'),
    });
    expect(fetch).toHaveBeenCalledWith('/api/v1/books/book-1/entries', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"type":"expense"'),
    });
    expect(fetch).toHaveBeenCalledWith('/api/v1/books/book-1/entries', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"amountCents":3000'),
    });
    expect(fetch).toHaveBeenCalledWith('/api/v1/books/book-1/entries', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"transactionCurrency":"USD"'),
    });
    expect(await screen.findByRole('region', { name: 'Home' })).toBeInTheDocument();
    const nav = await screen.findByRole('navigation', { name: 'Main navigation' });
    expect(within(nav).getByRole('button', { name: 'Home' })).toHaveAttribute('aria-current', 'page');
    await waitFor(() => expect(screen.getByRole('region', { name: 'Transactions' })).toHaveTextContent('Team lunch'));
  });

  it('opens grouped categories from the All record shortcut and edits a category there', async () => {
    setCurrentCategories([
      {
        ...fixtureCategory,
        id: 'category-parent-food',
        name: 'Food',
        sortOrder: -1,
      },
      {
        ...fixtureCategory,
        parentId: 'category-parent-food',
      },
      {
        ...fixtureCategory,
        id: 'category-fuel',
        name: 'Fuel',
        sortOrder: 1,
      },
      {
        ...fixtureCategory,
        id: 'category-office',
        name: 'Office',
        sortOrder: 2,
      },
    ]);

    renderApp();

    expect(await openRecordTab()).toBeInTheDocument();
    const shortcuts = await screen.findByRole('group', { name: 'Category shortcuts' });
    expect(within(shortcuts).getByRole('button', { name: 'All' })).toBeInTheDocument();
    fireEvent.click(within(shortcuts).getByRole('button', { name: 'All' }));

    const sheet = await screen.findByRole('region', { name: 'Select category' });
    expect(within(sheet).getByText('Food')).toBeInTheDocument();
    expect(within(sheet).getByRole('button', { name: /Dining/ })).toBeInTheDocument();
    fireEvent.click(within(sheet).getByRole('button', { name: /Fuel/ }));

    await waitFor(() => expect(screen.getByRole('region', { name: 'Selected category' })).toHaveTextContent('Fuel'));

    fireEvent.click(within(shortcuts).getByRole('button', { name: 'All' }));
    const reopenedSheet = await screen.findByRole('region', { name: 'Select category' });
    fireEvent.click(within(reopenedSheet).getByText('Manage categories'));
    const diningRow = within(reopenedSheet).getByLabelText('Name for Dining').closest('li') as HTMLElement;
    fireEvent.change(within(diningRow).getByLabelText('Name for Dining'), { target: { value: 'Meals' } });
    fireEvent.click(within(diningRow).getByRole('button', { name: 'Save category' }));

    expect(await screen.findByText('Category updated.')).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledWith('/api/v1/books/book-1/categories/category-1', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"name":"Meals"'),
    });
  });

  it('creates, renames, and archives a category from the record tab', async () => {
    renderApp();

    expect(await openRecordTab()).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole('region', { name: 'Categories' })).toBeInTheDocument());
    fireEvent.change(screen.getByLabelText('Category name'), { target: { value: 'Fuel' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create category' }));

    expect(await screen.findByText('Category created.')).toBeInTheDocument();
    expect(await screen.findByLabelText('Name for Fuel')).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledWith('/api/v1/books/book-1/categories', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"name":"Fuel"'),
    });
    expect(fetch).toHaveBeenCalledWith('/api/v1/books/book-1/categories', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"direction":"expense"'),
    });

    const fuelRow = screen.getByLabelText('Name for Fuel').closest('li') as HTMLElement;
    fireEvent.change(within(fuelRow).getByLabelText('Name for Fuel'), { target: { value: 'Road fuel' } });
    fireEvent.click(within(fuelRow).getByRole('button', { name: 'Save category' }));

    expect(await screen.findByText('Category updated.')).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledWith('/api/v1/books/book-1/categories/category-created', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"name":"Road fuel"'),
    });

    const renamedRow = await screen.findByLabelText('Name for Road fuel');
    const row = renamedRow.closest('li') as HTMLElement;
    fireEvent.click(within(row).getByRole('button', { name: 'Archive' }));
    fireEvent.click(within(row).getByRole('button', { name: 'Save category' }));

    expect(await screen.findByText('Category updated.')).toBeInTheDocument();
    expect(within(row).getByRole('button', { name: 'Restore' })).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledWith('/api/v1/books/book-1/categories/category-created', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"archived":true'),
    });
  });

  it('keeps transactions out of the record tab', async () => {
    renderApp();

    expect(await openRecordTab()).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole('group', { name: 'Category shortcuts' })).toHaveTextContent('Dining'));
    expect(screen.queryByRole('region', { name: 'Transactions' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Edit details' })).not.toBeInTheDocument();
    expect(screen.getByRole('region', { name: 'Categories' })).toBeInTheDocument();
  });

  it('searches saved transactions from the header search control', async () => {
    renderApp();

    expect(await openRecordTab()).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole('group', { name: 'Category shortcuts' })).toHaveTextContent('Dining'));
    fireEvent.click(screen.getByRole('button', { name: 'Search transactions' }));

    expect(await screen.findByRole('region', { name: 'Search transactions' })).toBeInTheDocument();
    await waitFor(() => expect(fetch).toHaveBeenCalledWith('/api/v1/books/book-1/entries?page=1&page_size=100'));
    fireEvent.change(screen.getByRole('textbox', { name: 'Search transactions' }), { target: { value: 'Converted' } });

    expect(await screen.findByRole('list', { name: 'Search results' })).toHaveTextContent('Converted lunch');
    expect(screen.getByRole('list', { name: 'Search results' })).toHaveTextContent('CN¥100.00');
    fireEvent.click(screen.getByRole('button', { name: 'Close search' }));
    expect(await screen.findByRole('region', { name: 'Record entry' })).toBeInTheDocument();
  });

  it('opens shared search URLs with the query already applied', async () => {
    renderApp('/search?query=Converted');

    expect(await screen.findByRole('region', { name: 'Search transactions' })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Search transactions' })).toHaveValue('Converted');
    expect(await screen.findByRole('list', { name: 'Search results' })).toHaveTextContent('Converted lunch');

    fireEvent.click(screen.getByRole('button', { name: 'Close search' }));
    expect(await screen.findByRole('region', { name: 'Home' })).toBeInTheDocument();
  });
});
