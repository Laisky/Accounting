import { expect, type Page } from '@playwright/test';

const PASSWORD = 'correct horse battery staple';

// registerAndSignIn creates a fresh account and establishes a session cookie through the API,
// so a test can deep-link straight into authenticated views without driving the login form.
export async function registerAndSignIn(page: Page, prefix = 'e2e'): Promise<string> {
  const email = `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2)}@example.test`;
  const register = await page.request.post('/api/auth/register', { data: { email, password: PASSWORD } });
  expect(register.status()).toBe(201);
  const login = await page.request.post('/api/auth/login', { data: { email, password: PASSWORD } });
  expect(login.status()).toBe(200);
  return email;
}
