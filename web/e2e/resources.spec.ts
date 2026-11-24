import { expect, test, type Page, type Route } from '@playwright/test';
import en from '../messages/en.json' with { type: 'json' };
import { OWNER, loginAsOwner } from './support/auth';
import { openNew } from './support/resources';

const secret = 'e2e-secret-value';

test('resource, credential, rule, binding and access check', async ({ page }) => {
	await loginAsOwner(page);
	await openNew(page);
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
	// the DB-IP credit shows only where its data is used, not on every page
	await expect(page.getByRole('link', { name: en.geoip_attribution })).toHaveCount(0);

	await page.getByRole('tab', { name: en.resources_tab_rules }).click();
	await expect(page.getByRole('link', { name: en.geoip_attribution })).toHaveAttribute('href', 'https://db-ip.com');
	await page.getByRole('button', { name: en.rules_add_network }).click();
	await page.getByLabel(en.rules_cidr).fill('10.0.0.0/8');
	await page.getByRole('button', { name: en.rules_save }).click();
	await expect(page.getByText(en.common_saved)).toBeVisible();

	await page.getByRole('tab', { name: en.resources_tab_access }).click();
	await page.getByLabel(en.access_subject, { exact: true }).selectOption({ label: `${OWNER.name} (${OWNER.login})` });
	await page.getByRole('button', { name: en.access_add }).click();
	await expect(page.getByRole('cell', { name: en.access_this_resource })).toBeVisible();

	await page.getByLabel(en.access_user, { exact: true }).selectOption({ label: `${OWNER.name} (${OWNER.login})` });
	await page.getByLabel(en.access_check_ip).fill('10.1.2.3');
	await page.getByRole('button', { name: en.access_check_run }).click();
	await expect(page.getByText(en.access_allowed, { exact: true })).toBeVisible();
	await page.getByLabel(en.access_check_ip).fill('192.0.2.10');
	await page.getByRole('button', { name: en.access_check_run }).click();
	await expect(page.getByText(en.access_denied, { exact: true })).toBeVisible();
	await expect(page.getByText(/^rule:network:/)).toBeVisible();

	await expect(page.getByRole('link', { name: en.geoip_attribution })).toHaveAttribute('href', 'https://db-ip.com');
});

test('open resource by keyboard', async ({ page }) => {
	await loginAsOwner(page);
	await openNew(page);
	await page.getByLabel(en.resources_name).fill('e2e-kbd');
	await page.getByLabel(en.resources_ssh_host).fill('10.0.0.7');
	await page.getByLabel(en.resources_ssh_login).fill('root');
	await page.getByRole('button', { name: en.resources_save, exact: true }).click();
	await expect(page).toHaveURL(/\/resources\/[0-9a-f-]{36}$/);
	const id = page.url().split('/').pop();
	await page.goto('/resources');
	await page.getByRole('link', { name: 'e2e-kbd' }).focus();
	await page.keyboard.press('Enter');
	await expect(page).toHaveURL(new RegExp(`/resources/${id}$`));
});

test('ssh remote forwarding toggle is saved', async ({ page }) => {
	await loginAsOwner(page);
	await openNew(page);
	await page.getByLabel(en.resources_name).fill('e2e-ssh-r');
	await page.getByLabel(en.resources_ssh_host).fill('10.0.0.8');
	await page.getByLabel(en.resources_ssh_login).fill('root');
	const toggle = page.getByRole('switch', { name: en.resources_ssh_remote_forwarding });
	await expect(toggle).not.toBeChecked();
	await toggle.locator('xpath=ancestor::label').click();
	await page.getByRole('button', { name: en.resources_save, exact: true }).click();
	await expect(page).toHaveURL(/\/resources\/[0-9a-f-]{36}$/);
	await page.reload();
	await expect(toggle).toBeChecked();
	await page.screenshot({ path: '../.superpowers/sdd/stage15-wave1/screens/t12-resource-remote-forwarding.png' });
	await page.getByRole('button', { name: en.resources_delete }).click();
	await page.getByRole('dialog').getByRole('button', { name: en.resources_delete }).click();
	await expect(page).toHaveURL(/\/resources$/);
});

test('tcp resource with ports, port binding, hints and health', async ({ page }) => {
	await loginAsOwner(page);
	await openNew(page, 'tcp');
	await page.getByLabel(en.resources_name).fill('e2e-tcp');
	await page.getByLabel(en.resources_ssh_host).fill('10.0.0.6');
	await page.getByLabel(en.resources_ports, { exact: true }).fill('0');
	await expect(page.getByText(en.resources_ports_invalid)).toBeVisible();
	await expect(page.getByRole('button', { name: en.resources_save, exact: true })).toBeDisabled();
	await page.getByLabel(en.resources_ports, { exact: true }).fill('22, 8000-8100');
	await page.getByRole('button', { name: en.resources_save, exact: true }).click();
	await expect(page).toHaveURL(/\/resources\/[0-9a-f-]{36}$/);
	await expect(page.getByText(/^ssh -L <port>:e2e-tcp:<port> /)).toBeVisible();
	await expect(page.getByText(/^werbot connect --gateway /)).toBeVisible();

	await page.getByRole('tab', { name: en.resources_tab_access }).click();
	await page.getByLabel(en.access_subject, { exact: true }).selectOption({ label: `${OWNER.name} (${OWNER.login})` });
	await page.getByLabel(en.access_ports).fill('22');
	await page.getByRole('button', { name: en.access_add }).click();
	await expect(page.getByRole('cell', { name: '22', exact: true })).toBeVisible();

	await page.goto('/resources');
	await expect(page.getByRole('row', { name: /e2e-tcp/ }).getByRole('img', { name: en.resources_health_unknown })).toBeVisible();
});

test.describe('pool list load on the resource form', () => {
	const listGateways = '**/werbot.v1.GatewayService/ListGateways';
	const fail = (route: Route, code: string) =>
		route.fulfill({ status: code === 'permission_denied' ? 403 : 500, contentType: 'application/json', body: JSON.stringify({ code, message: code }) });

	async function fillValid(page: Page) {
		await page.getByLabel(en.resources_name).fill('e2e-pool-load');
		await page.getByLabel(en.resources_ssh_host).fill('10.0.0.5');
		await page.getByLabel(en.resources_ssh_login).fill('root');
	}

	test.beforeEach(async ({ page }) => {
		await loginAsOwner(page);
	});

	test('other error shows a banner', async ({ page }) => {
		await page.route(listGateways, (r) => fail(r, 'internal'));
		await openNew(page);
		await expect(page.getByText(en.error_unknown)).toBeVisible();
	});

	test('Save stays disabled until the pool list settles', async ({ page }) => {
		let release!: () => void;
		const gate = new Promise<void>((r) => (release = r));
		await page.route(listGateways, async (r) => {
			await gate;
			await r.continue();
		});
		await openNew(page);
		await fillValid(page);
		const save = page.getByRole('button', { name: en.resources_save, exact: true });
		await expect(save).toBeDisabled();
		release();
		await expect(save).toBeEnabled();
	});

	test('permission denied shows no banner and still saves', async ({ page }) => {
		await page.route(listGateways, (r) => fail(r, 'permission_denied'));
		await openNew(page);
		await fillValid(page);
		await expect(page.getByRole('button', { name: en.resources_save, exact: true })).toBeEnabled();
		await expect(page.getByText(en.error_permission_denied)).toHaveCount(0);
		await expect(page.getByText(en.error_unknown)).toHaveCount(0);
	});
});
