---
id: playwright-e2e
version: 1.0.0
team: qa
stack: Playwright, TypeScript, Jest, Supertest
---

# Scroll: E2E Testing — Playwright & Supertest

## Triggers — load when:
- Files: `playwright.config.ts`, `e2e/**`, `**/*.e2e.ts`, `**/*.e2e-spec.ts`
- Keywords: playwright, e2e, end-to-end, browser test, page object, visual regression, supertest, HTTP test, smoke test, integration test

## Rules

### 1. Page Object Model — always use for pages with multiple tests

```typescript
// e2e/pages/insurance-selector.page.ts
import { Page, Locator } from '@playwright/test';

export class InsuranceSelectorPage {
  readonly offerCards: Locator;
  readonly continueButton: Locator;
  readonly loadingIndicator: Locator;
  readonly errorAlert: Locator;
  readonly retryButton: Locator;

  constructor(private page: Page) {
    this.offerCards     = page.getByTestId('offer-card');
    this.continueButton = page.getByTestId('continue-btn');
    this.loadingIndicator = page.getByTestId('skeleton');
    this.errorAlert     = page.getByRole('alert');
    this.retryButton    = page.getByTestId('retry-btn');
  }

  async goto() {
    await this.page.goto('/insurance/select');
    await this.waitForContent();
  }

  async waitForContent() {
    // Wait for loading to finish — never arbitrary sleep
    await this.page.waitForSelector('[data-testid="offer-card"], [role="alert"]');
  }

  async selectOffer(index: number) {
    await this.offerCards.nth(index).click();
  }

  async proceed() {
    await this.continueButton.click();
  }
}
```

### 2. API mocking — always mock external services in E2E

```typescript
// e2e/fixtures/api-mocks.ts
import { Page } from '@playwright/test';

export async function mockInsuranceApi(page: Page, scenario: 'success' | 'empty' | 'error') {
  await page.route('**/insurance/v1/offers**', route => {
    switch (scenario) {
      case 'success':
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            offers: [
              { id: '1', name: 'Plan Básico', monthlyPremium: 9990, currency: 'CLP' },
              { id: '2', name: 'Plan Completo', monthlyPremium: 19990, currency: 'CLP' },
            ],
          }),
        });
      case 'empty':
        return route.fulfill({ status: 200, body: JSON.stringify({ offers: [] }) });
      case 'error':
        return route.fulfill({ status: 503, body: JSON.stringify({ error: 'Unavailable' }) });
    }
  });
}
```

### 3. Test structure — complete suite

```typescript
// e2e/insurance-selection.spec.ts
import { test, expect } from '@playwright/test';
import { InsuranceSelectorPage } from './pages/insurance-selector.page';
import { mockInsuranceApi } from './fixtures/api-mocks';

test.describe('Insurance selection flow', () => {
  let page: InsuranceSelectorPage;

  test.beforeEach(async ({ page: p }) => {
    page = new InsuranceSelectorPage(p);
  });

  test.describe('Happy path', () => {
    test.beforeEach(async ({ page: p }) => {
      await mockInsuranceApi(p, 'success');
      await page.goto();
    });

    test('displays 2 offer cards', async () => {
      await expect(page.offerCards).toHaveCount(2);
    });

    test('continue button disabled until selection', async () => {
      await expect(page.continueButton).toBeDisabled();
    });

    test('selecting offer enables continue button', async () => {
      await page.selectOffer(0);
      await expect(page.continueButton).toBeEnabled();
    });

    test('proceed navigates to confirmation', async ({ page: p }) => {
      await page.selectOffer(0);
      await page.proceed();
      await expect(p).toHaveURL('/insurance/confirm');
    });
  });

  test.describe('Empty state', () => {
    test('shows empty state message when no offers', async ({ page: p }) => {
      await mockInsuranceApi(p, 'empty');
      await page.goto();
      await expect(p.getByText(/No hay planes disponibles/)).toBeVisible();
    });
  });

  test.describe('Error state', () => {
    test('shows error alert and retry button on API failure', async ({ page: p }) => {
      await mockInsuranceApi(p, 'error');
      await page.goto();
      await expect(page.errorAlert).toBeVisible();
      await expect(page.retryButton).toBeVisible();
    });

    test('retry loads offers on second attempt', async ({ page: p }) => {
      await mockInsuranceApi(p, 'error');
      await page.goto();
      // Fix the mock to return success on retry
      await mockInsuranceApi(p, 'success');
      await page.retryButton.click();
      await expect(page.offerCards).toHaveCount(2);
    });
  });

  test.describe('Accessibility', () => {
    test('offer cards are keyboard navigable', async ({ page: p }) => {
      await mockInsuranceApi(p, 'success');
      await page.goto();
      await p.keyboard.press('Tab');
      const focused = p.locator(':focus');
      await expect(focused).toHaveAttribute('data-testid', 'offer-card');
    });
  });
});
```

### 4. Playwright config — multi-browser, CI-aware

```typescript
// playwright.config.ts
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  expect: { timeout: 5_000 },
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 4 : undefined,
  reporter: [
    ['html', { open: 'never' }],
    ['junit', { outputFile: 'e2e-results.xml' }],
    process.env.CI ? ['github'] : ['list'],
  ],
  use: {
    baseURL: process.env.E2E_BASE_URL || 'http://localhost:3000',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
    { name: 'mobile-safari', use: { ...devices['iPhone 13'] } },
  ],
});
```

### 5. data-testid convention (mandatory)

```html
<!-- Every interactive element tested by E2E gets data-testid -->
<design-cards data-testid="offer-card" />
<design-button data-testid="continue-btn">Continuar</design-button>
<div data-testid="skeleton" *ngIf="isLoading()" />
<design-alerts data-testid="error-alert" role="alert" />
<design-button data-testid="retry-btn">Reintentar</design-button>

<!-- Never use CSS classes or text for E2E selectors -->
<!-- ❌ page.locator('.primary-btn') — breaks on style changes -->
<!-- ❌ page.getByText('Continuar') — breaks on i18n -->
<!-- ✅ page.getByTestId('continue-btn') — stable -->
```

---

## Anti-patterns

```typescript
// ❌ Arbitrary sleep — flaky tests
await page.waitForTimeout(2000);
// ✅ Explicit wait
await page.waitForSelector('[data-testid="offer-card"]');

// ❌ Calling real APIs in E2E
// ✅ Always mock with page.route()

// ❌ Tests that depend on each other
test('step 1', () => { ... });
test('step 2 (assumes step 1 ran)', () => { ... });

// ❌ Hardcoded element text for selectors
page.getByText('Continuar')  // breaks on language change

// ❌ No retries in CI (network/rendering variability)
retries: 0  // in CI environment
```
