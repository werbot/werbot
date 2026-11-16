import { spawn, type ChildProcess } from 'node:child_process';
import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { expect, test } from '@playwright/test';
import en from '../messages/en.json' with { type: 'json' };
import { loginAsOwner } from './support/auth';

const procs: ChildProcess[] = [];
test.afterAll(() => procs.forEach((p) => p.kill()));

test('pool with two instances, join token and revocation', async ({ page, baseURL }) => {
	test.setTimeout(120_000);
	await loginAsOwner(page);
	await page.getByRole('button', { name: en.nav_gateways }).click();
	await page.getByLabel(en.gateways_name_label).fill('e2e-pool');
	await page.getByRole('button', { name: en.gateways_create }).click();
	await page.getByRole('button', { name: 'e2e-pool' }).click();

	await page.getByRole('button', { name: en.gateways_token_create }).click();
	const dialog = page.getByRole('dialog');
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

	const poolSection = (name: string) => page.locator('section').filter({ has: page.getByRole('button', { name, exact: true }) });
	const pool = poolSection('e2e-pool');
	await expect(pool.getByText(en.gateways_online, { exact: true }).first()).toBeVisible({ timeout: 30_000 });
	for (const i of [1, 2]) {
		await expect(pool.getByRole('row', { name: new RegExp(`e2e-gw-${i}.*${en.gateways_online}`) })).toBeVisible({ timeout: 30_000 });
	}
	await expect(pool.getByText('2 of 2')).toBeVisible({ timeout: 30_000 });

	await pool.getByRole('button', { name: en.gateways_token_revoke }).click();
	await expect(pool.getByText(en.gateways_token_revoked, { exact: true })).toBeVisible();

	procs[1].kill();
	await pool.getByRole('row', { name: /e2e-gw-2/ }).getByRole('button', { name: en.gateways_instance_delete }).click();
	await page.getByRole('dialog').getByRole('button', { name: en.gateways_instance_delete }).click();
	await expect(pool.getByRole('row', { name: /e2e-gw-2/ })).toHaveCount(0);

	// resource form with two pools: pool select, connect hint, pool change persists
	await page.getByLabel(en.gateways_name_label).fill('e2e-pool-2');
	await page.getByRole('button', { name: en.gateways_create }).click();
	await expect(poolSection('e2e-pool-2')).toBeVisible();

	const poolSelect = page.locator('.miso-select', { hasText: en.resources_gateway_label }).getByRole('button');
	const pickPool = async (name: string) => {
		await poolSelect.click();
		await page.getByRole('menuitem', { name, exact: true }).click();
	};
	await page.goto('/resources/new');
	await expect(poolSelect).toBeVisible();
	await page.getByLabel(en.resources_name).fill('e2e-pool-res');
	await page.getByLabel(en.resources_ssh_host).fill('10.0.0.5');
	await page.getByLabel(en.resources_ssh_login).fill('root');
	await pickPool('e2e-pool');
	await page.getByRole('button', { name: en.resources_save, exact: true }).click();
	await expect(page).toHaveURL(/\/resources\/[0-9a-f-]{36}$/);
	await expect(page.locator('code', { hasText: '+e2e-pool-res@' })).toBeVisible();

	await pickPool('e2e-pool-2');
	await page.getByRole('button', { name: en.resources_save, exact: true }).click();
	await expect(page.getByText(en.common_saved)).toBeVisible();
	await page.reload();
	await expect(poolSelect).toHaveText(/e2e-pool-2/);

	await page.getByRole('button', { name: en.resources_delete }).click();
	await page.getByRole('dialog').getByRole('button', { name: en.resources_delete }).click();
	await expect(page).toHaveURL(/\/resources$/);

	procs[0].kill();
	await page.goto('/gateways');
	for (const name of ['e2e-pool', 'e2e-pool-2']) {
		await poolSection(name).getByRole('button', { name: en.gateways_delete }).click();
		await page.getByRole('dialog').getByRole('button', { name: en.gateways_delete }).click();
		await expect(poolSection(name)).toHaveCount(0);
	}
});
