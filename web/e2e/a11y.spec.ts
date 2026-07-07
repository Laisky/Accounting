import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';
import { registerAndSignIn } from './helpers';

const VIEWS = [
  { path: '/home', region: 'Home' },
  { path: '/accounts', region: 'Accounts' },
  { path: '/record', region: 'Record entry' },
  { path: '/reports/category', region: 'Reports' },
  { path: '/me', region: 'Me' },
];

for (const theme of ['light', 'dark'] as const) {
  test(`core authenticated views have no serious axe violations (${theme})`, async ({ page }) => {
    test.setTimeout(90_000);
    await page.addInitScript((mode) => window.localStorage.setItem('accountingTheme', mode), theme);
    await registerAndSignIn(page, `a11y-${theme}`);

    for (const view of VIEWS) {
      await page.goto(view.path);
      await expect(page.getByRole('region', { name: view.region })).toBeVisible();
      const results = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa']).analyze();
      const serious = results.violations.filter((v) => v.impact === 'serious' || v.impact === 'critical');
      expect(serious.map((v) => `${v.id}@${view.path}`)).toEqual([]);
    }
  });
}
