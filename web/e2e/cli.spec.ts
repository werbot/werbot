import { execFile, spawn } from 'node:child_process';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { expect, test } from '@playwright/test';
import en from '../messages/en.json' with { type: 'json' };
import { loginAsOwner } from './support/auth';
import { rpc } from './support/rpc';

const BIN = '../bin/werbot';
let home = ''; // WERBOT_HOME of the CLI under test
test.afterAll(() => {
	if (home) rmSync(home, { recursive: true, force: true });
});

// run resolves with the exit code and output of file, run with the CLI's profile home.
function run(file: string, args: string[]): Promise<{ code: number; stdout: string; stderr: string }> {
	return new Promise((resolve) => {
		execFile(file, args, { env: { ...process.env, WERBOT_HOME: home }, timeout: 20_000 }, (err, stdout, stderr) =>
			resolve({ code: err ? (typeof err.code === 'number' ? err.code : -1) : 0, stdout, stderr })
		);
	});
}
const werbot = (...args: string[]) => run(BIN, args);

test('werbot login --device, werbot ssh, then revoked in the profile', async ({ page, baseURL }) => {
	test.setTimeout(120_000);
	home = mkdtempSync(join(tmpdir(), 'werbot-cli-'));

	// the CLI prints the code, the owner approves it in the browser, the CLI saves the profile
	const login = spawn(BIN, ['login', '--device', '--server', baseURL!], { env: { ...process.env, WERBOT_HOME: home } });
	let out = '';
	login.stdout.on('data', (b) => (out += b));
	const exited = new Promise<number | null>((r) => login.on('exit', r));
	await expect.poll(() => /enter code (\S+)/.exec(out)?.[1] ?? '', { timeout: 10_000 }).not.toBe('');
	const userCode = /enter code (\S+)/.exec(out)![1];

	await loginAsOwner(page);
	await page.goto(`/device?user_code=${encodeURIComponent(userCode)}`);
	await page.getByRole('button', { name: en.device_approve }).click();
	await expect(page.getByText(en.device_done)).toBeVisible();
	expect(await exited).toBe(0);

	// the "local" pool gets its SSH address; a resource on the e2e sshtarget is bound to the owner
	const { gateways } = await rpc(page, 'werbot.v1.GatewayService/ListGateways', {});
	const local = gateways.find((g: { name: string }) => g.name === 'local');
	await rpc(page, 'werbot.v1.GatewayService/UpdateGateway', { id: local.id, sshAddress: '127.0.0.1:2222' });
	const { resource } = await rpc(page, 'werbot.v1.ResourceService/CreateResource', {
		name: 'e2e-cli',
		type: 'ssh',
		gatewayId: local.id,
		config: { '@type': 'type.googleapis.com/werbot.connector.ssh.v1.Config', host: '127.0.0.1', port: 2022, login: 'e2e' }
	});
	await rpc(page, 'werbot.v1.ResourceService/SetCredential', { resourceId: resource.id, kind: 'password', value: btoa('e2e-secret') });
	const { user } = await rpc(page, 'werbot.v1.AuthService/GetSession', {});
	await rpc(page, 'werbot.v1.AccessService/CreateBinding', { subjectType: 'user', subjectId: user.id, resourceId: resource.id });

	// the gateway sees the new resource after its snapshot debounce
	const probe = () => werbot('ssh', 'e2e-cli', '--', 'echo', 'werbot-probe');
	await expect.poll(async () => (await probe()).stdout, { timeout: 30_000 }).toContain('werbot-probe');
	expect((await probe()).code).toBe(0);
	const sshConfig = join(home, 'ssh_config');
	writeFileSync(sshConfig, (await werbot('ssh-config')).stdout);

	await page.goto('/profile/cli-sessions');
	const row = page.getByRole('row').filter({ has: page.getByRole('button', { name: en.common_more }) });
	await expect(row).toHaveCount(1);
	await row.getByRole('button', { name: en.common_more }).click();
	await page.getByRole('menuitem', { name: en.cli_sessions_revoke }).click();
	await page.getByRole('dialog', { name: en.cli_sessions_revoke }).getByRole('button', { name: en.cli_sessions_revoke }).click();
	await expect(page.getByText(en.cli_sessions_empty)).toBeVisible();

	// werbot ssh stops at the API, which refuses the revoked token
	const after = await probe();
	expect(after.code).not.toBe(0);
	expect(after.stderr).toContain('session expired or revoked; run werbot login');
	// OpenSSH still holds a certificate inside its hour; the gateway refuses its revoked session
	const openssh = await run('ssh', ['-F', sshConfig, '-o', 'BatchMode=yes', 'e2e-cli.127.0.0.1', 'echo', 'werbot-probe']);
	expect(openssh.code).toBe(255);
	expect(openssh.stderr).toContain('Permission denied');
});
