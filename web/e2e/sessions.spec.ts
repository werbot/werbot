import { execFileSync } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { expect, test } from '@playwright/test';
import en from '../messages/en.json' with { type: 'json' };
import { loginAsOwner } from './support/auth';
import { rpc } from './support/rpc';

let dir = ''; // holds the generated private key
test.afterAll(() => {
	if (dir) rmSync(dir, { recursive: true, force: true });
});

test('ssh session is listed, replayed and audited', async ({ page }) => {
	test.setTimeout(90_000);
	await loginAsOwner(page);
	dir = mkdtempSync(join(tmpdir(), 'werbot-e2e-'));
	execFileSync('ssh-keygen', ['-q', '-t', 'ed25519', '-N', '', '-f', join(dir, 'id')]);
	await rpc(page, 'werbot.v1.SSHKeyService/AddSSHKey', { name: 'e2e', publicKey: readFileSync(join(dir, 'id.pub'), 'utf8') });
	const { gateways } = await rpc(page, 'werbot.v1.GatewayService/ListGateways', {});
	const local = gateways.find((g: { name: string }) => g.name === 'local');
	expect(local).toBeTruthy();
	const { resource } = await rpc(page, 'werbot.v1.ResourceService/CreateResource', {
		name: 'e2e-target',
		type: 'ssh',
		gatewayId: local.id,
		config: { '@type': 'type.googleapis.com/werbot.connector.ssh.v1.Config', host: '127.0.0.1', port: 2022, login: 'e2e' }
	});
	await rpc(page, 'werbot.v1.ResourceService/SetCredential', { resourceId: resource.id, kind: 'password', value: btoa('e2e-secret') });
	const { user } = await rpc(page, 'werbot.v1.AuthService/GetSession', {});
	await rpc(page, 'werbot.v1.AccessService/CreateBinding', { subjectType: 'user', subjectId: user.id, resourceId: resource.id });

	// the gateway sees the new resource after its snapshot debounce
	const ssh = () => {
		try {
			return execFileSync(
				'ssh',
				['-F', '/dev/null', '-i', join(dir, 'id'), '-p', '2222', '-o', 'BatchMode=yes', '-o', 'IdentitiesOnly=yes',
					'-o', 'ConnectTimeout=3', '-o', 'StrictHostKeyChecking=no', '-o', 'UserKnownHostsFile=/dev/null',
					`${user.login}+e2e-target@127.0.0.1`, 'echo werbot-e2e-marker'],
				// each attempt fits several times into the poll window
				{ encoding: 'utf8', timeout: 5_000, stdio: ['ignore', 'pipe', 'pipe'] }
			);
		} catch {
			return '';
		}
	};
	await expect.poll(ssh, { timeout: 15_000 }).toContain('werbot-e2e-marker');

	await page.goto('/sessions');
	await expect(page.getByRole('columnheader', { name: en.sessions_col_traffic })).toBeVisible();
	await page.getByRole('link', { name: 'e2e-target' }).first().click();
	await expect(page).toHaveURL(/\/sessions\/[0-9a-f-]{36}$/);
	const castURL = `/api/recordings/${page.url().split('/').pop()}.cast`;
	await expect
		.poll(async () => (await page.request.get(castURL)).headers()['x-werbot-recording-verified'], { timeout: 20_000 })
		.toBe('true');
	const cast = await (await page.request.get(castURL)).text();
	expect(JSON.parse(cast.split('\n')[0]).version).toBe(2);
	expect(cast).toContain('werbot-e2e-marker');
	await page.reload();
	await expect(page.getByText(en.sessions_recording_verified)).toBeVisible();
	await expect(page.getByTestId('cast-player').locator('.ap-player')).toBeVisible();

	const recordings = '**/api/recordings/*.cast';
	await page.route(recordings, (r) => r.fulfill({ headers: { 'X-Werbot-Recording-Verified': 'false' }, body: cast }));
	await page.reload();
	await expect(page.getByText(en.sessions_recording_failed)).toBeVisible();
	await page.unroute(recordings);
	await page.route(recordings, (r) => r.fulfill({ status: 500 }));
	await page.reload();
	await expect(page.getByText(en.sessions_recording_load_error)).toBeVisible();
	await page.unroute(recordings);

	await page.goto('/audit?action=session');
	await expect(page.getByText('session.start').first()).toBeVisible();
	await expect(page.getByText('session.end').first()).toBeVisible();
	await page.getByText('session.start').first().click();
	await expect(page.getByRole('dialog').getByText(resource.id)).toBeVisible();
});
