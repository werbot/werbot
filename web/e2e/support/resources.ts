import type { Page } from '@playwright/test';
import en from '../../messages/en.json' with { type: 'json' };

// walks the wizard: category tile, then type tile
export async function openNew(page: Page, type = 'ssh', category = en.resources_category_servers) {
	await page.goto('/resources');
	await page.getByRole('button', { name: en.resources_new }).click();
	await page.getByRole('button', { name: category, exact: true }).click();
	await page.getByRole('button', { name: type, exact: true }).click();
}
