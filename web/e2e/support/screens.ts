import type { Page } from '@playwright/test';

// shot saves a screenshot for review under .superpowers/<path> only when
// SCREENS=1; no assertion depends on it.
export async function shot(page: Page, path: string): Promise<void> {
	if (process.env.SCREENS === '1') await page.screenshot({ path: `../.superpowers/${path}` });
}
