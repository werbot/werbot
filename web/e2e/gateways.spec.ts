import { spawn, type ChildProcess } from 'node:child_process';
import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { expect, test, type Page } from '@playwright/test';
import en from '../messages/en.json' with { type: 'json' };
import { choose } from './support/select';
import { loginAsOwner } from './support/auth';
import { openNew } from './support/resources';
import { removeNamed } from './support/rpc';
import { shot } from './support/screens';

const procs: ChildProcess[] = [];
test.afterAll(() => procs.forEach((p) => p.kill()));
// a failed step leaves the WebSocket pool behind
test.afterEach(({ page }) => removeNamed(page, 'GatewayService', 'Gateway', 'gateways', 'e2e-ws-pool'));

async function createPool(page: Page, name: string) {
	await page.getByRole('button', { name: en.gateways_create }).click();
	const modal = page.getByRole('dialog', { name: en.gateways_create });
	await modal.getByLabel(en.gateways_name_label).fill(name);
	await modal.getByRole('button', { name: en.gateways_create }).click();
	await expect(modal).toHaveCount(0);
}

const poolRow = (page: Page, name: string) => page.getByRole('row', { name: new RegExp(`^${name}\\s`) });

test('pool with two instances, join token and revocation', async ({ page, baseURL }) => {
	test.setTimeout(120_000);
	await loginAsOwner(page);
	await page.getByRole('link', { name: en.nav_access }).click();
	await page.getByRole('link', { name: en.nav_gateways }).click();
	await createPool(page, 'e2e-pool');
	await page.getByRole('button', { name: 'e2e-pool', exact: true }).click();
	const pool = page.getByRole('dialog', { name: 'e2e-pool', exact: true });

	await page.getByRole('button', { name: en.gateways_token_create }).click();
	const dialog = page.getByRole('dialog', { name: en.gateways_token_create, exact: true });
	await dialog.getByLabel(en.gateways_token_expires_in).fill('1');
	await dialog.getByLabel(en.gateways_token_max_uses).fill('2');
	await dialog.getByRole('button', { name: en.gateways_token_create }).click();
	const command = (await dialog.locator('code').textContent()) ?? '';
	expect(command).toMatch(/^WERBOT_GATEWAY_JOIN_TOKEN=wjt_\S+ werbot gateway --server \S+$/); // token not in argv
	const token = command.match(/wjt_\S+/)?.[0];
	expect(token).toBeTruthy();
	await dialog.getByRole('button', { name: 'Close', exact: true }).click();

	for (const i of [1, 2]) {
		const p = spawn(
			'../bin/werbot',
			['gateway', '--server', baseURL!, '--join-token', token!, '--data-dir', mkdtempSync(join(tmpdir(), `e2e-gw-${i}-`)), '--ssh-addr', '127.0.0.1:0', '--name', `e2e-gw-${i}`],
			{ stdio: 'ignore' }
		);
		procs.push(p);
	}

	await expect(pool.getByText(en.gateways_online, { exact: true }).first()).toBeVisible({ timeout: 30_000 });
	for (const i of [1, 2]) {
		await expect(pool.getByRole('row', { name: new RegExp(`e2e-gw-${i}.*${en.gateways_online}`) })).toBeVisible({ timeout: 30_000 });
	}
	await expect(pool.getByText('2 of 2')).toBeVisible({ timeout: 30_000 });

	await pool.getByRole('button', { name: en.gateways_token_revoke }).click();
	await expect(pool.getByText(en.gateways_token_revoked, { exact: true })).toBeVisible();

	procs[1].kill();
	await pool.getByRole('row', { name: /e2e-gw-2/ }).getByRole('button', { name: en.common_more }).click();
	// Escape closes only the instance menu, not the drawer
	await page.keyboard.press('Escape');
	await expect(page.getByRole('menuitem', { name: en.gateways_instance_delete })).toBeHidden();
	await expect(pool).toBeVisible();
	await pool.getByRole('row', { name: /e2e-gw-2/ }).getByRole('button', { name: en.common_more }).click();
	await page.getByRole('menuitem', { name: en.gateways_instance_delete }).click();
	await page.getByRole('dialog', { name: en.gateways_instance_delete, exact: true }).getByRole('button', { name: en.gateways_instance_delete }).click();
	await expect(pool.getByRole('row', { name: /e2e-gw-2/ })).toHaveCount(0);

	// resource form with two pools: pool select, connect hint, pool change persists
	await page.keyboard.press('Escape');
	await expect(pool).toHaveCount(0);
	await createPool(page, 'e2e-pool-2');
	await expect(poolRow(page, 'e2e-pool-2')).toBeVisible();

	const poolSelect = page.getByLabel(en.resources_gateway_label);
	const pickPool = async (name: string) => {
		await choose(poolSelect, name);
	};
	await openNew(page);
	await expect(poolSelect).toBeVisible();
	await page.getByLabel(en.resources_name).fill('e2e-pool-res');
	await page.getByLabel(en.resources_ssh_host).fill('10.0.0.5');
	await page.getByLabel(en.resources_ssh_login).fill('root');
	await pickPool('e2e-pool');
	await page.getByRole('button', { name: en.resources_save, exact: true }).click();
	await expect(page).toHaveURL(/\/resources\/[0-9a-f-]{36}$/);
	await expect(page.locator('code', { hasText: '+e2e-pool-res@' })).toBeVisible();

	await pickPool('e2e-pool-2');
	// wait for the answer: the "Saved" toast of the create may still be visible
	const updated = page.waitForResponse('**/werbot.v1.ResourceService/UpdateResource');
	await page.getByRole('button', { name: en.resources_save, exact: true }).click();
	expect((await updated).ok()).toBe(true);
	await page.reload();
	await expect(poolSelect).toHaveText('e2e-pool-2');

	await page.getByRole('button', { name: en.resources_delete }).click();
	await page.getByRole('dialog').getByRole('button', { name: en.resources_delete }).click();
	await expect(page).toHaveURL(/\/resources$/);

	procs[0].kill();
	await page.goto('/gateways');
	for (const name of ['e2e-pool', 'e2e-pool-2']) {
		await deletePoolFromRow(page, name);
		await expect(poolRow(page, name)).toHaveCount(0);
	}
});

async function deletePoolFromRow(page: Page, name: string) {
	await poolRow(page, name).getByRole('button', { name: en.common_more }).click();
	await page.getByRole('menuitem', { name: en.gateways_delete }).click();
	await page.getByRole('dialog', { name: en.gateways_delete, exact: true }).getByRole('button', { name: en.gateways_delete }).click();
}

test('delete pool from drawer', async ({ page }) => {
	await loginAsOwner(page);
	await page.goto('/gateways');
	await createPool(page, 'e2e-drawer-pool');
	await page.getByRole('button', { name: 'e2e-drawer-pool', exact: true }).click();
	const drawer = page.getByRole('dialog', { name: 'e2e-drawer-pool', exact: true });
	await expect(drawer.getByRole('heading', { name: en.gateways_instances_title })).toBeVisible();
	await drawer.getByRole('button', { name: en.gateways_token_create }).click();
	const tokenDialog = page.getByRole('dialog', { name: en.gateways_token_create, exact: true });
	await expect(tokenDialog).toBeVisible();
	await page.keyboard.press('Escape'); // closes only the token dialog
	await expect(tokenDialog).toHaveCount(0);
	await expect(drawer).toBeVisible();
	await drawer.getByRole('button', { name: en.common_more }).click();
	await page.getByRole('menuitem', { name: en.gateways_delete }).click();
	const confirm = page.getByRole('dialog', { name: en.gateways_delete, exact: true });
	await page.keyboard.press('Escape'); // closes only the confirm
	await expect(confirm).toHaveCount(0);
	await expect(drawer).toBeVisible();
	await drawer.getByRole('button', { name: en.common_more }).click();
	await page.getByRole('menuitem', { name: en.gateways_delete }).click();
	await confirm.getByRole('button', { name: en.gateways_delete }).click();
	await expect(drawer).toHaveCount(0);
	await expect(poolRow(page, 'e2e-drawer-pool')).toHaveCount(0);
});

test('pool WebSocket URL is saved and a bad scheme is refused', async ({ page }) => {
	await loginAsOwner(page);
	await page.goto('/gateways');
	await createPool(page, 'e2e-ws-pool');
	await page.getByRole('button', { name: 'e2e-ws-pool', exact: true }).click();
	const drawer = page.getByRole('dialog', { name: 'e2e-ws-pool', exact: true });
	const field = drawer.getByLabel(en.gateways_ws_url_label);

	await field.fill('http://x');
	await drawer.getByRole('button', { name: en.gateways_ssh_address_save }).click();
	await expect(page.getByText(/ws_url must be a ws:\/\/ or wss:\/\/ URL/)).toBeVisible();

	await field.fill('wss://gw.example.com/v1/ssh');
	await drawer.getByRole('button', { name: en.gateways_ssh_address_save }).click();
	await expect(page.getByText(en.common_saved).first()).toBeVisible();
	await shot(page, 'sdd/stage15-wave1/screens/t12-pool-ws-url.png');
	await page.reload();
	await page.getByRole('button', { name: 'e2e-ws-pool', exact: true }).click();
	await expect(page.getByRole('dialog', { name: 'e2e-ws-pool', exact: true }).getByLabel(en.gateways_ws_url_label)).toHaveValue('wss://gw.example.com/v1/ssh');

	// restore: empty URL, then delete the pool
	await field.fill('');
	await drawer.getByRole('button', { name: en.gateways_ssh_address_save }).click();
	await expect(page.getByText(en.common_saved).first()).toBeVisible();
	await page.keyboard.press('Escape');
	await deletePoolFromRow(page, 'e2e-ws-pool');
	await expect(poolRow(page, 'e2e-ws-pool')).toHaveCount(0);
});
