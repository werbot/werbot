import { expect, test, type Page } from '@playwright/test';
import en from '../messages/en.json' with { type: 'json' };
import { loginAsOwner } from './support/auth';
import { openNew } from './support/resources';

const presetRow = (page: Page) => page.getByRole('row', { name: /PostgreSQL/ });

test('preset becomes a type, its resource shows the launch hint, a used preset stays', async ({ page }) => {
	await loginAsOwner(page);
	await page.getByRole('link', { name: en.nav_access }).click();
	await page.getByRole('link', { name: en.nav_presets }).click();
	await expect(page).toHaveURL(/\/resources\/presets$/);

	await page.getByRole('button', { name: en.presets_create }).first().click();
	const drawer = page.getByRole('dialog', { name: en.presets_create });
	await drawer.getByLabel(en.presets_name_label, { exact: true }).fill('postgres');
	await drawer.getByLabel(en.presets_display_name_label).fill('PostgreSQL');
	await drawer.getByLabel(en.presets_ports_label).fill('5432');
	await drawer.getByLabel(en.presets_launch_hint_label).fill('psql -h {host} -p {port}');
	await page.waitForTimeout(600); // drawer slide-in, for the screenshot only
	await page.screenshot({ path: '../.superpowers/sdd/stage15-wave1/screens/t12-preset-drawer.png' });
	await drawer.getByRole('button', { name: en.resources_save, exact: true }).click();
	await expect(presetRow(page)).toBeVisible();
	await page.screenshot({ path: '../.superpowers/sdd/stage15-wave1/screens/t12-presets-list.png' });

	await openNew(page, 'PostgreSQL', en.resources_category_databases);
	await expect(page.getByLabel(en.resources_ports, { exact: true })).toHaveValue('5432');
	await page.getByLabel(en.resources_name).fill('pg');
	await page.getByLabel(en.resources_ssh_host).fill('db.internal');
	await page.getByRole('button', { name: en.resources_save, exact: true }).click();
	await expect(page).toHaveURL(/\/resources\/[0-9a-f-]{36}$/);
	await expect(page.getByText('Preset: PostgreSQL')).toBeVisible();
	await expect(page.locator('code', { hasText: 'psql -h 127.0.0.1 -p 5432' })).toBeVisible();
	await page.screenshot({ path: '../.superpowers/sdd/stage15-wave1/screens/t12-resource-preset.png' });
	const resourceUrl = page.url();

	await page.goto('/resources/presets');
	await presetRow(page).getByRole('button', { name: en.common_more }).click();
	await page.getByRole('menuitem', { name: en.presets_delete }).click();
	await page.getByRole('dialog', { name: en.presets_delete, exact: true }).getByRole('button', { name: en.presets_delete }).click();
	await expect(page.getByText(en.presets_delete_in_use)).toBeVisible();
	await expect(presetRow(page)).toBeVisible();

	// cleanup: the resource first, then the preset
	await page.goto(resourceUrl);
	await page.getByRole('button', { name: en.resources_delete }).click();
	await page.getByRole('dialog').getByRole('button', { name: en.resources_delete }).click();
	await expect(page).toHaveURL(/\/resources$/);
	await page.goto('/resources/presets');
	await presetRow(page).getByRole('button', { name: en.common_more }).click();
	await page.getByRole('menuitem', { name: en.presets_delete }).click();
	await page.getByRole('dialog', { name: en.presets_delete, exact: true }).getByRole('button', { name: en.presets_delete }).click();
	await expect(presetRow(page)).toHaveCount(0);
});
