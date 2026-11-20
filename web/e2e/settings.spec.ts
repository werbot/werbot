import { expect, test } from '@playwright/test';
import en from '../messages/en.json' with { type: 'json' };
import { BOB, loginAsOwner, loginWithNext } from './support/auth';

// durations are text ("12h", "30d") since stage 10 (owner ruling O2): one style for all six fields
test('owner changes the CLI session idle timeout; a bad ceiling is refused', async ({ page }) => {
	await loginAsOwner(page);
	await page.getByRole('link', { name: en.nav_organization }).click();
	// exact: dashboard audit rows ("settings.update" since requests.spec) also match "Settings"
	await page.getByRole('link', { name: en.nav_settings, exact: true }).click();
	await expect(page).toHaveURL(/\/settings$/);

	const ttl = page.getByLabel(en.settings_cli_session_ttl);
	const max = page.getByLabel(en.settings_cli_session_max);
	const save = page.getByRole('button', { name: en.settings_save }).first();
	await ttl.fill('12h');
	await save.click();
	await expect(page.getByText(en.settings_saved)).toBeVisible();
	await page.reload();
	await expect(ttl).toHaveValue('12h');

	const before = await max.inputValue();
	await max.fill('2x');
	await save.click();
	await expect(page.getByText(en.common_duration_invalid)).toBeVisible();
	await max.fill('0s');
	await save.click();
	await expect(page.getByText(en.error_invalid_argument)).toBeVisible();
	await page.reload();
	await expect(max).toHaveValue(before);
	await expect(ttl).toHaveValue('12h');
});

test('a member has no Settings in the sidebar', async ({ page }) => {
	await page.goto('/login');
	await loginWithNext(page, BOB);
	await expect(page).toHaveURL('/');
	await page.goto('/profile');
	await expect(page.getByRole('link', { name: en.cli_sessions_title })).toBeVisible();
	await expect(page.getByRole('link', { name: en.nav_organization })).toHaveCount(0);
	await expect(page.getByRole('link', { name: en.nav_settings })).toHaveCount(0);
});

test('pool SSH address persists; the host key is shown', async ({ page }) => {
	await loginAsOwner(page);
	await page.goto('/gateways');
	await page.getByRole('button', { name: en.gateways_create }).click();
	const modal = page.getByRole('dialog', { name: en.gateways_create });
	await modal.getByLabel(en.gateways_name_label).fill('e2e-addr-pool');
	await modal.getByRole('button', { name: en.gateways_create }).click();
	await expect(modal).toHaveCount(0);

	const open = async () => {
		await page.getByRole('button', { name: 'e2e-addr-pool', exact: true }).click();
		return page.getByRole('dialog', { name: 'e2e-addr-pool', exact: true });
	};
	let drawer = await open();
	await expect(drawer.locator('code.host-key')).toHaveText(/^ssh-ed25519 /);
	await drawer.getByLabel(en.gateways_ssh_address_label).fill('127.0.0.1:2222');
	await drawer.getByRole('button', { name: en.gateways_ssh_address_save }).click();
	await expect(page.getByText(en.common_saved)).toBeVisible();

	await page.reload();
	drawer = await open();
	await expect(drawer.getByLabel(en.gateways_ssh_address_label)).toHaveValue('127.0.0.1:2222');

	await drawer.getByRole('button', { name: en.common_more }).click();
	await page.getByRole('menuitem', { name: en.gateways_delete }).click();
	await page.getByRole('dialog', { name: en.gateways_delete, exact: true }).getByRole('button', { name: en.gateways_delete }).click();
	await expect(drawer).toHaveCount(0);
});
