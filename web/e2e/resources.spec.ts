import { expect, test } from '@playwright/test';
import en from '../messages/en.json' with { type: 'json' };
import { OWNER, loginAsOwner } from './support/auth';

const secret = 'e2e-secret-value';

test('resource, credential, rule, binding and access check', async ({ page }) => {
	await loginAsOwner(page);
	await page.goto('/resources/new');
	await page.getByLabel(en.resources_name).fill('e2e-ssh');
	await page.getByLabel(en.resources_ssh_host).fill('10.0.0.5');
	await page.getByLabel(en.resources_ssh_login).fill('root');
	await page.getByRole('button', { name: en.resources_save, exact: true }).click();
	await expect(page).toHaveURL(/\/resources\/[0-9a-f-]{36}$/);

	await page.getByRole('tab', { name: en.resources_tab_credentials }).click();
	await page.getByLabel(en.cred_kind_password, { exact: true }).fill(secret);
	await page.getByRole('button', { name: en.cred_save }).click();
	await expect(page.getByText(en.cred_set_password)).toBeVisible();
	await page.reload();
	await page.getByRole('tab', { name: en.resources_tab_credentials }).click();
	await expect(page.getByText(en.cred_set_password)).toBeVisible();
	await expect(page.locator('body')).not.toContainText(secret);
	await expect(page.locator('input').evaluateAll((els) => els.map((e) => (e as HTMLInputElement).value))).resolves.not.toContain(secret);

	await page.getByRole('tab', { name: en.resources_tab_rules }).click();
	await page.getByRole('button', { name: en.rules_add_network }).click();
	await page.getByLabel(en.rules_cidr).fill('10.0.0.0/8');
	await page.getByRole('button', { name: en.rules_save }).click();
	await expect(page.getByText(en.common_saved)).toBeVisible();

	await page.getByRole('tab', { name: en.resources_tab_access }).click();
	await page.getByRole('button', { name: 'Select…' }).first().click();
	await page.getByRole('menuitem', { name: new RegExp(OWNER.login) }).click();
	await page.getByRole('button', { name: en.access_add }).click();
	await expect(page.getByRole('cell', { name: en.access_this_resource })).toBeVisible();

	await page.getByRole('button', { name: 'Select…' }).click();
	await page.getByRole('menuitem', { name: new RegExp(OWNER.login) }).last().click();
	await page.getByLabel(en.access_check_ip).fill('10.1.2.3');
	await page.getByRole('button', { name: en.access_check_run }).click();
	await expect(page.getByText(en.access_allowed, { exact: true })).toBeVisible();
	await page.getByLabel(en.access_check_ip).fill('192.0.2.10');
	await page.getByRole('button', { name: en.access_check_run }).click();
	await expect(page.getByText(en.access_denied, { exact: true })).toBeVisible();
	await expect(page.getByText(/^rule:network:/)).toBeVisible();

	await expect(page.getByRole('link', { name: en.geoip_attribution })).toHaveAttribute('href', 'https://db-ip.com');
});
