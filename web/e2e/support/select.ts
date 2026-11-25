import { expect, type Locator } from '@playwright/test';

// choose opens a FilledSelect (found by its label), picks the option with this exact text
// and waits until the menu has gone, so the next choose sees only its own menu.
export async function choose(field: Locator, option: string) {
	const page = field.page();
	await field.click();
	await page.getByRole('menuitem', { name: option, exact: true }).click();
	await expect(page.getByRole('menu')).toHaveCount(0);
}
