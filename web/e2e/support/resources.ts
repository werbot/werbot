import type { Page } from '@playwright/test';
import en from '../../messages/en.json' with { type: 'json' };

export async function openNew(page: Page) {
	await page.goto('/resources');
	await page.getByRole('button', { name: en.resources_new }).click();
}
