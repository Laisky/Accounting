import { createHmac } from 'node:crypto';
import { expect, test, type CDPSession, type Page } from '@playwright/test';

// generateTotpCode receives an otpauth URI and offset and returns the six-digit code for browser acceptance tests.
function generateTotpCode(otpauth: string, offsetMs = 0): string {
  const secret = new URL(otpauth).searchParams.get('secret');
  if (!secret) {
    throw new Error('otpauth URI missing secret');
  }

  const counter = Math.floor((Date.now() + offsetMs) / 30000);
  const message = Buffer.alloc(8);
  message.writeBigUInt64BE(BigInt(counter));
  const hmac = createHmac('sha1', decodeBase32(secret)).update(message).digest();
  const offset = hmac[hmac.length - 1] & 0x0f;
  const binary = ((hmac[offset] & 0x7f) << 24) | (hmac[offset + 1] << 16) | (hmac[offset + 2] << 8) | hmac[offset + 3];

  return String(binary % 1000000).padStart(6, '0');
}

// decodeBase32 receives a base32 TOTP secret and returns its raw bytes.
function decodeBase32(value: string): Buffer {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  const normalized = value.toUpperCase().replace(/=+$/u, '').replace(/\s+/gu, '');
  let bits = 0;
  let bitLength = 0;
  const bytes: number[] = [];

  for (const char of normalized) {
    const index = alphabet.indexOf(char);
    if (index < 0) {
      throw new Error('invalid base32 secret');
    }
    bits = (bits << 5) | index;
    bitLength += 5;
    if (bitLength >= 8) {
      bytes.push((bits >> (bitLength - 8)) & 0xff);
      bitLength -= 8;
    }
  }

  return Buffer.from(bytes);
}

type VirtualAuthenticator = {
  authenticatorId: string;
  client: CDPSession;
};

// addPasskeyAuthenticator receives a page and adds a CTAP2 virtual authenticator for passkey tests.
async function addPasskeyAuthenticator(page: Page): Promise<VirtualAuthenticator> {
  const client = await page.context().newCDPSession(page);
  await client.send('WebAuthn.enable', { enableUI: false });
  const response = (await client.send('WebAuthn.addVirtualAuthenticator', {
    options: {
      protocol: 'ctap2',
      ctap2Version: 'ctap2_1',
      transport: 'internal',
      hasResidentKey: true,
      hasUserVerification: true,
      automaticPresenceSimulation: true,
      isUserVerified: true,
    },
  })) as { authenticatorId: string };
  await client.send('WebAuthn.setUserVerified', {
    authenticatorId: response.authenticatorId,
    isUserVerified: true,
  });

  return { authenticatorId: response.authenticatorId, client };
}

// removePasskeyAuthenticator receives a virtual authenticator and tears down the CDP WebAuthn state.
async function removePasskeyAuthenticator(authenticator: VirtualAuthenticator): Promise<void> {
  await authenticator.client.send('WebAuthn.removeVirtualAuthenticator', {
    authenticatorId: authenticator.authenticatorId,
  });
  await authenticator.client.send('WebAuthn.disable');
}

test('user can start external SSO from the authentication screen', async ({ page }) => {
  await page.goto('/');

  await expect(page.getByRole('heading', { name: 'Enter the ledger with an auditable identity.' })).toBeVisible();
  await expect(page.getByText('External SSO')).toBeVisible();
  await page.getByRole('link', { name: 'Use SSO' }).click();

  await expect(page).toHaveURL(/\/api\/health\?redirect_to=/);
});

test('user can register and request recovery from the authentication screen', async ({ page }) => {
  const email = `ui-register-${Date.now()}-${Math.random().toString(36).slice(2)}@example.test`;
  const password = 'correct horse battery staple';

  await page.goto('/');
  await page.getByRole('button', { name: 'Register' }).click();
  await page.getByLabel('Email').fill(email);
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Create account' }).click();
  await expect(page.getByText('Registration complete. Sign in to continue.')).toBeVisible();

  await page.getByRole('button', { name: 'Recover' }).click();
  await page.getByLabel('Email').fill(email);
  await page.getByRole('button', { name: 'Send reset email' }).click();
  await expect(page.getByText('Password reset email requested.')).toBeVisible();
  await expect(page.getByText(/reset code:|verification code:/i)).toHaveCount(0);
});

test.describe('passkeys', () => {
  test('user registers, renames, signs in with, and deletes a passkey', async ({ page }) => {
    const authenticator = await addPasskeyAuthenticator(page);
    const email = `passkey-${Date.now()}-${Math.random().toString(36).slice(2)}@example.test`;
    const password = 'correct horse battery staple';

    try {
      const registerResponse = await page.request.post('/api/auth/register', {
        data: { email, password },
      });
      expect(registerResponse.status()).toBe(201);

      const loginResponse = await page.request.post('/api/auth/login', {
        data: { email, password },
      });
      expect(loginResponse.status()).toBe(200);

      await page.goto('/');
      await page.getByRole('navigation', { name: 'Main navigation' }).getByRole('button', { name: 'Me' }).click();
      const passkeys = page.getByRole('article', { name: 'Passkeys' });
      await expect(passkeys.getByText('Registered passkeys: 0')).toBeVisible();
      await page.getByLabel('Passkey label').fill('Acceptance passkey');
      await passkeys.getByRole('button', { name: 'Register passkey' }).click();
      await expect(page.getByText('Passkey registered.')).toBeVisible();
      await expect(page.getByLabel('Label for Acceptance passkey')).toBeVisible();

      await page.getByLabel('Label for Acceptance passkey').fill('Renamed acceptance passkey');
      await passkeys.getByRole('button', { name: 'Rename' }).click();
      await expect(page.getByText('Passkey renamed.')).toBeVisible();
      await expect(page.getByLabel('Label for Renamed acceptance passkey')).toBeVisible();

      await page.getByRole('button', { name: 'Sign out' }).click();
      await expect(page.getByRole('heading', { name: 'Enter the ledger with an auditable identity.' })).toBeVisible();
      await page.getByRole('button', { name: 'Use passkey' }).click();
      await expect(page.getByRole('region', { name: 'Record entry' })).toBeVisible();

      await page.getByRole('navigation', { name: 'Main navigation' }).getByRole('button', { name: 'Me' }).click();
      await expect(page.getByLabel('Label for Renamed acceptance passkey')).toBeVisible();
      await passkeys.getByRole('button', { name: 'Delete' }).click();
      await expect(page.getByText('Passkey deleted.')).toBeVisible();
      await expect(page.getByText('No passkeys registered yet.')).toBeVisible();

      await page.getByRole('button', { name: 'Load activity' }).click();
      await expect(page.getByText('auth / passkey_deleted')).toBeVisible();
      await expect(page.getByText('auth / passkey_login')).toBeVisible();
      await expect(page.getByText('auth / passkey_renamed')).toBeVisible();
      await expect(page.getByText('auth / passkey_registered')).toBeVisible();
    } finally {
      await removePasskeyAuthenticator(authenticator);
    }
  });
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
  test.setTimeout(60000);

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
  await expect(nav).toBeInViewport();
  await nav.getByRole('button', { name: 'Accounts' }).click();
  await expect(page.getByRole('region', { name: 'Accounts' })).toBeVisible();
  await page.getByRole('button', { name: 'Prepare account' }).click();
  await expect(page.getByText('Account ready.')).toBeVisible();
  await expect(page.getByRole('article', { name: 'Book members' }).getByText('owner')).toBeVisible();
  await page.getByLabel('Account name').fill('Travel wallet');
  await page.getByLabel('Opening balance').fill('123.45');
  await page.getByRole('button', { name: 'Create account' }).click();
  await expect(page.getByText('Account created.')).toBeVisible();
  await expect(page.getByText('Travel wallet')).toBeVisible();
  await expect(page.getByRole('listitem').filter({ hasText: 'Travel wallet' }).getByText('$123.45')).toBeVisible();
  await expect(nav).toBeInViewport();
  await page.getByLabel('Base currency').selectOption('EUR');
  await expect(page.getByText('Base currency updated.')).toBeVisible();
  await page.getByLabel('Base currency').selectOption('USD');
  await expect(page.getByText('Base currency updated.')).toBeVisible();
  await page.getByLabel('Book name').fill('Household 2026');
  await page.getByRole('button', { name: 'Save book' }).click();
  await expect(page.getByText('Book updated.')).toBeVisible();
  await expect(page.getByLabel('Book name')).toHaveValue('Household 2026');
  await page.getByLabel('Account group name').fill('Daily Wallets');
  await page.getByRole('button', { name: 'Save group' }).click();
  await expect(page.getByText('Account group updated.')).toBeVisible();
  await expect(page.getByLabel('Account group name')).toHaveValue('Daily Wallets');

  await nav.getByRole('button', { name: 'Record' }).click();
  const expenseTab = page.getByRole('tab', { name: 'Expense' });
  await expect(expenseTab).toHaveAttribute('aria-selected', 'true');
  await expect(expenseTab).toBeInViewport();
  expect(await page.evaluate(() => {
    const content = document.querySelector('.mobileContent');
    return Boolean(content && content.scrollWidth <= content.clientWidth);
  })).toBe(true);
  await page.getByLabel('Category name').fill('Fuel');
  await page.getByRole('button', { name: 'Create category' }).click();
  await expect(page.getByText('Category created.')).toBeVisible();
  await expect(page.getByLabel('Name for Fuel')).toBeVisible();
  const fuelRow = page.getByRole('listitem').filter({ has: page.getByLabel('Name for Fuel') });
  await fuelRow.getByLabel('Name for Fuel').fill('Road fuel');
  await fuelRow.getByRole('button', { name: 'Save category' }).click();
  await expect(page.getByText('Category updated.')).toBeVisible();
  const roadFuelRow = page.getByRole('listitem').filter({ has: page.getByLabel('Name for Road fuel') });
  await roadFuelRow.getByRole('button', { name: 'Archive' }).click();
  await roadFuelRow.getByRole('button', { name: 'Save category' }).click();
  await expect(page.getByText('Category updated.')).toBeVisible();
  await expect(roadFuelRow.getByRole('button', { name: 'Restore' })).toBeVisible();
  await page.getByLabel('Category name').fill('Dining');
  await page.getByRole('button', { name: 'Create category' }).click();
  await expect(page.getByText('Category created.')).toBeVisible();
  await page.getByRole('button', { name: '2' }).click();
  await page.getByRole('button', { name: '4' }).click();
  await page.getByPlaceholder('Add a note...').fill('Team lunch');
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByText('Entry posted.')).toBeVisible();
  await expect(page.getByLabel('Transaction Team lunch')).toBeVisible();

  await nav.getByRole('button', { name: 'Reports' }).click();
  await expect(page.getByRole('region', { name: 'Reports' })).toBeVisible();
  await page.getByRole('button', { name: 'Report filters' }).click();
  await expect(page.getByLabel('Active report filters').getByText('Total')).toBeVisible();
  await expect(page.getByText('$24.00', { exact: true })).toBeVisible();
  await expect(page.getByText('1 entries', { exact: true })).toBeVisible();

  await nav.getByRole('button', { name: 'Record' }).click();
  await page.getByRole('button', { name: 'Edit details' }).click();
  await page.getByLabel('Amount for Team lunch').fill('45.67');
  await page.getByLabel('Time for Team lunch').fill('2026-07-02T09:15');
  await page.getByLabel('Account for Team lunch').selectOption({ label: 'Travel wallet' });
  await page.getByLabel('Note for Team lunch').fill('Team lunch edited');
  await page.getByLabel('Merchant for Team lunch').fill('Corner Cafe');
  await page.getByLabel('Tags for Team lunch').fill('food, work');
  await page.getByRole('button', { name: 'Save details' }).click();
  await expect(page.getByText('Entry updated.')).toBeVisible();
  await expect(page.getByText('Team lunch edited')).toBeVisible();
  await expect(page.getByText('$45.67')).toBeVisible();
  await page.getByRole('button', { name: 'Search transactions' }).click();
  await expect(page.getByRole('region', { name: 'Search transactions' })).toBeVisible();
  await page.getByRole('textbox', { name: 'Search transactions' }).fill('edited');
  await expect(page.getByRole('list', { name: 'Search results' }).getByText('Team lunch edited')).toBeVisible();
  await expect(page.getByRole('list', { name: 'Search results' }).getByText('$45.67')).toBeVisible();
  await expect(page.getByRole('list', { name: 'Search results' }).getByText(/Travel wallet/)).toBeVisible();
  await page.getByRole('button', { name: 'Close search' }).click();
  await expect(page.getByRole('region', { name: 'Record entry' })).toBeVisible();
  await page.getByRole('button', { name: 'Delete entry' }).click();
  await expect(page.getByText('Entry deleted.')).toBeVisible();
  await expect(page.getByText('Team lunch edited')).toHaveCount(0);

  await nav.getByRole('button', { name: 'Import' }).click();
  await expect(page.getByRole('region', { name: 'Import data' })).toBeVisible();
  await page.getByLabel('Upload Wacai export file').setInputFiles({
    name: 'wacai.csv',
    mimeType: 'text/csv',
    buffer: Buffer.from(
      'date,type,amount,currency,account,category,book,member,merchant,note,tags\n2026-07-01,expense,12.30,cny,Cash,Dining,Household,Alice,Market,Import lunch,food|work\n',
    ),
  });
  await expect(page.getByText('wacai.csv')).toBeVisible();
  await page.getByRole('button', { name: 'Stage import' }).click();
  await expect(page.getByText('Import staged')).toBeVisible();
  await expect(page.getByLabel('Import preview summary').getByText('Rows')).toBeVisible();
  await expect(page.getByLabel('Detected import values').getByText('Cash')).toBeVisible();
  await expect(page.getByLabel('Detected import values').getByText('Dining')).toBeVisible();
  await expect(page.getByLabel('Import row diagnostics').getByText('Import lunch')).toBeVisible();
  await expect(page.getByLabel('Import row diagnostics').getByText('CNY 12.30')).toBeVisible();
  await page.getByRole('button', { name: 'Apply import' }).click();
  await expect(page.getByText('Imported 1 rows, skipped 0.')).toBeVisible();
  await expect(page.getByText('Import applied.')).toBeVisible();

  await nav.getByRole('button', { name: 'Record' }).click();
  await page.getByRole('button', { name: 'Search transactions' }).click();
  await page.getByRole('textbox', { name: 'Search transactions' }).fill('Import lunch');
  await expect(page.getByRole('list', { name: 'Search results' }).getByText('Import lunch')).toBeVisible();
  await expect(page.getByRole('list', { name: 'Search results' }).getByText('CN¥12.30')).toBeVisible();
  await page.getByRole('button', { name: 'Close search' }).click();

  await nav.getByRole('button', { name: 'Me' }).click();
  await expect(page.getByRole('region', { name: 'Me' })).toBeVisible();
  await expect(page.getByText(email)).toBeVisible();
  await expect(page.getByRole('article', { name: 'Authenticator app' }).getByText('Authenticator app is off.')).toBeVisible();
  await page.getByRole('button', { name: 'Set up TOTP' }).click();
  await expect(page.getByText('TOTP setup started.')).toBeVisible();
  const otpauth = await page.getByLabel('Authenticator setup URI').inputValue();
  await page.getByLabel('TOTP code').fill(generateTotpCode(otpauth));
  await page.getByRole('button', { name: 'Confirm TOTP' }).click();
  await expect(page.getByText('TOTP enabled.')).toBeVisible();
  await expect(page.getByRole('article', { name: 'Authenticator app' }).getByText('Authenticator app is on.')).toBeVisible();
  await page.getByRole('button', { name: 'Sign out' }).click();
  await expect(page.getByRole('heading', { name: 'Enter the ledger with an auditable identity.' })).toBeVisible();
  await page.getByLabel('Email').fill(email);
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in with email' }).click();
  await expect(page.getByText('Enter the code from your authenticator app to finish signing in.')).toBeVisible();
  await page.getByLabel('TOTP code').fill(generateTotpCode(otpauth));
  await page.getByRole('button', { name: 'Verify code' }).click();
  await expect(page.getByRole('region', { name: 'Record entry' })).toBeVisible();
  const postTotpNav = page.getByRole('navigation', { name: 'Main navigation' });
  await postTotpNav.getByRole('button', { name: 'Me' }).click();
  await page.getByLabel('TOTP code').fill(generateTotpCode(otpauth, -30000));
  await page.getByRole('button', { name: 'Disable TOTP' }).click();
  await expect(page.getByText('TOTP disabled.')).toBeVisible();
  await page.getByRole('button', { name: 'Load activity' }).click();
  await expect(page.getByText('auth / totp_disabled')).toBeVisible();
  await expect(page.getByText('auth / login_totp_challenge')).toBeVisible();
  await expect(page.getByText('auth / totp_enabled')).toBeVisible();
  await expect(page.getByText('auth / totp_setup_requested')).toBeVisible();
  await expect(page.getByText('import / committed')).toBeVisible();
  await expect(page.getByText('entry / created').first()).toBeVisible();
  await expect(page.getByText(password)).toHaveCount(0);
  await page.getByRole('button', { name: 'Sign out' }).click();
  await expect(page.getByRole('heading', { name: 'Enter the ledger with an auditable identity.' })).toBeVisible();
  const sessionResponse = await page.request.get('/api/auth/session');
  expect(sessionResponse.status()).toBe(401);
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
