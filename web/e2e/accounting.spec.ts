import { expect, test } from '@playwright/test';

test('user can start external SSO from the authentication screen', async ({ page }) => {
  await page.goto('/');

  await expect(page.getByRole('heading', { name: 'Enter the ledger with an auditable identity.' })).toBeVisible();
  await expect(page.getByText('External SSO')).toBeVisible();
  await page.getByRole('link', { name: 'Use SSO' }).click();

  await expect(page).toHaveURL(/\/api\/health\?redirect_to=/);
});

test('user signs in through the authentication screen', async ({ page }) => {
  const email = `ui-login-${Date.now()}-${Math.random().toString(36).slice(2)}@example.test`;
  const password = 'correct horse battery staple';

  const registerResponse = await page.request.post('/api/auth/register', {
    data: { email, password },
  });
  expect(registerResponse.status()).toBe(201);

  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Enter the ledger with an auditable identity.' })).toBeVisible();
  await page.getByLabel('Email').fill(email);
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in with email' }).click();

  await expect(page.getByRole('region', { name: 'Record entry' })).toBeVisible();
  await expect(page.getByRole('navigation', { name: 'Main navigation' })).toBeVisible();
});

test('authenticated user uses the mobile accounting tabs', async ({ page }) => {
  const email = `mobile-tabs-${Date.now()}-${Math.random().toString(36).slice(2)}@example.test`;
  const password = 'correct horse battery staple';

  const registerResponse = await page.request.post('/api/auth/register', {
    data: { email, password },
  });
  expect(registerResponse.status()).toBe(201);

  const loginResponse = await page.request.post('/api/auth/login', {
    data: { email, password },
  });
  expect(loginResponse.status()).toBe(200);

  const auditResponse = await page.request.get('/api/audit?page=1&page_size=10');
  expect(auditResponse.status()).toBe(200);
  const auditBody = await auditResponse.json();
  const auditActions = auditBody.items.map((item: { action: string }) => item.action);
  expect(auditActions).toContain('auth.register');
  expect(auditActions).toContain('auth.login');
  expect(JSON.stringify(auditBody)).not.toContain(password);
  expect(JSON.stringify(auditBody)).not.toContain('token');

  await page.goto('/');
  await expect(page.getByRole('region', { name: 'Record entry' })).toBeVisible();

  const nav = page.getByRole('navigation', { name: 'Main navigation' });
  await nav.getByRole('button', { name: 'Accounts' }).click();
  await expect(page.getByRole('region', { name: 'Accounts' })).toBeVisible();
  await page.getByRole('button', { name: 'Prepare account' }).click();
  await expect(page.getByText('Account ready.')).toBeVisible();

  await nav.getByRole('button', { name: 'Record' }).click();
  await expect(page.getByRole('tab', { name: 'Expense' })).toHaveAttribute('aria-selected', 'true');
  await page.getByRole('button', { name: '2' }).click();
  await page.getByRole('button', { name: '4' }).click();
  await page.getByLabel('Note').fill('Team lunch');
  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText('Entry posted.')).toBeVisible();

  await nav.getByRole('button', { name: 'Me' }).click();
  await expect(page.getByRole('region', { name: 'Me' })).toBeVisible();
  await expect(page.getByText(email)).toBeVisible();
});

test('auth recovery endpoints return generic non-secret responses', async ({ request }) => {
  const email = `recovery-${Date.now()}-${Math.random().toString(36).slice(2)}@example.test`;
  const password = 'correct horse battery staple';

  const registerResponse = await request.post('/api/auth/register', {
    data: { email, password },
  });
  expect(registerResponse.status()).toBe(201);

  const verificationResponse = await request.get(`/api/auth/email/verification?email=${encodeURIComponent(email)}`);
  expect(verificationResponse.status()).toBe(202);
  const verificationBody = await verificationResponse.json();
  expect(verificationBody.status).toBe('sent');
  expect(verificationBody.code).toBeUndefined();

  const resetResponse = await request.post('/api/auth/password-reset/request', {
    data: { email },
  });
  expect(resetResponse.status()).toBe(202);
  const resetBody = await resetResponse.json();
  expect(resetBody.status).toBe('sent');
  expect(resetBody.code).toBeUndefined();

  const unknownResetResponse = await request.post('/api/auth/password-reset/request', {
    data: { email: `missing-${email}` },
  });
  expect(unknownResetResponse.status()).toBe(202);
  const unknownResetBody = await unknownResetResponse.json();
  expect(unknownResetBody.status).toBe('sent');
  expect(unknownResetBody.code).toBeUndefined();
});
