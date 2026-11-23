import { execFileSync, spawn } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { expect, test, type Page } from '@playwright/test';
import en from '../messages/en.json' with { type: 'json' };
import { loginAsOwner } from './support/auth';
import { rpc } from './support/rpc';

const ssh = 'type.googleapis.com/werbot.connector.ssh.v1.Config';
const telnet = 'type.googleapis.com/werbot.connector.telnet.v1.Config';

let dir = ''; // holds the generated private key
test.afterAll(() => {
	if (dir) rmSync(dir, { recursive: true, force: true });
});

// the embedded gateway registers its "local" pool shortly after the server starts
async function localGateway(page: Page): Promise<string> {
	const pool = async () =>
		((await rpc(page, 'werbot.v1.GatewayService/ListGateways', {})).gateways ?? []).find((g: { name: string }) => g.name === 'local');
	await expect.poll(pool, { timeout: 30_000 }).toBeTruthy();
	return (await pool()).id;
}

// a resource with a password and a binding for the owner; the binding comes last
async function addResource(page: Page, name: string, type: string, config: object, password: string): Promise<string> {
	const { resource } = await rpc(page, 'werbot.v1.ResourceService/CreateResource', {
		name,
		type,
		gatewayId: await localGateway(page),
		config: { '@type': type === 'ssh' ? ssh : telnet, host: '127.0.0.1', ...config }
	});
	await rpc(page, 'werbot.v1.ResourceService/SetCredential', { resourceId: resource.id, kind: 'password', value: btoa(password) });
	const { user } = await rpc(page, 'werbot.v1.AuthService/GetSession', {});
	await rpc(page, 'werbot.v1.AccessService/CreateBinding', { subjectType: 'user', subjectId: user.id, resourceId: resource.id });
	return resource.id;
}

// opens the terminal from the resource list, waits for `ready` (the target's greeting) and types a line; until the gateway
// sees the new resource (snapshot debounce) the stream closes, so retry
async function typeInTerminal(page: Page, name: string, line: string, marker: string, ready = '') {
	await expect(async () => {
		await page.goto('/resources');
		await page.getByRole('row').filter({ hasText: name }).getByRole('button', { name: en.terminal_open }).click();
		await expect(page).toHaveURL(`/terminal/${name}`);
		await page.getByTestId('xterm').click();
		if (ready) await expect(page.getByTestId('xterm')).toContainText(ready, { timeout: 8_000 });
		await page.keyboard.type(line);
		await page.keyboard.press('Enter');
		await expect(page.getByTestId('xterm')).toContainText(marker, { timeout: 5_000 });
	}).toPass({ timeout: 40_000 });
}

async function webSession(page: Page, resource: string) {
	let found: { id: string; front: string } | undefined;
	await expect
		.poll(
			async () => {
				const { sessions } = await rpc(page, 'werbot.v1.SessionService/ListSessions', {});
				found = (sessions ?? []).find((s: { resourceName: string; front: string }) => s.resourceName === resource && s.front === 'web');
				return found?.id;
			},
			{ timeout: 20_000 }
		)
		.toBeTruthy();
	return found!.id;
}

// the recorded output as one string: the cast holds one event per chunk
async function recording(page: Page, id: string): Promise<string> {
	const url = `/api/recordings/${id}.cast`;
	await expect
		.poll(async () => (await page.request.get(url)).headers()['x-werbot-recording-verified'], { timeout: 20_000 })
		.toBe('true');
	return (await (await page.request.get(url)).text())
		.split('\n')
		.slice(1)
		.filter(Boolean)
		.map((l) => JSON.parse(l))
		.filter((e) => e[1] === 'o')
		.map((e) => e[2])
		.join('');
}

test('web terminal to ssh and telnet is recorded', async ({ page }) => {
	test.setTimeout(150_000);
	await loginAsOwner(page);
	await addResource(page, 'e2e-web-ssh', 'ssh', { port: 2022, login: 'e2e' }, 'e2e-secret');
	await addResource(page, 'e2e-web-telnet', 'telnet', { port: 2023, login: 'e2e' }, 'e2e-telnet');

	await typeInTerminal(page, 'e2e-web-ssh', 'echo werbot-web-marker', 'werbot-web-marker');
	await page.goto('/'); // leaving the page closes the stream and ends the session
	const sshCast = await recording(page, await webSession(page, 'e2e-web-ssh'));
	expect(sshCast).toContain('werbot-web-marker');

	await typeInTerminal(page, 'e2e-web-telnet', 'echo werbot-telnet-marker', 'werbot-telnet-marker', 'Welcome');
	// the password goes to the target only, never to the browser
	await expect(page.getByTestId('xterm')).not.toContainText('e2e-telnet');
	await page.goto('/');
	const telnetCast = await recording(page, await webSession(page, 'e2e-web-telnet'));
	expect(telnetCast).toContain('werbot-telnet-marker');
	expect(telnetCast).not.toContain('e2e-telnet');
});

test('live view follows an ssh session and terminate ends it', async ({ page }) => {
	test.setTimeout(120_000);
	await loginAsOwner(page);
	dir = mkdtempSync(join(tmpdir(), 'werbot-e2e-'));
	execFileSync('ssh-keygen', ['-q', '-t', 'ed25519', '-N', '', '-f', join(dir, 'id')]);
	await rpc(page, 'werbot.v1.SSHKeyService/AddSSHKey', { name: 'e2e-live', publicKey: readFileSync(join(dir, 'id.pub'), 'utf8') });
	const { user } = await rpc(page, 'werbot.v1.AuthService/GetSession', {});

	const run = () => {
		const p = spawn('ssh', [
			'-tt', '-F', '/dev/null', '-i', join(dir, 'id'), '-p', '2222', '-o', 'BatchMode=yes', '-o', 'IdentitiesOnly=yes',
			'-o', 'ConnectTimeout=3', '-o', 'StrictHostKeyChecking=no', '-o', 'UserKnownHostsFile=/dev/null',
			`${user.login}+e2e-web-ssh@127.0.0.1`
		]);
		const out = { p, stdout: '', done: false };
		p.stdout.on('data', (b) => (out.stdout += b));
		p.on('close', () => (out.done = true));
		return out;
	};
	const active = async () => {
		const { sessions } = await rpc(page, 'werbot.v1.SessionService/ListSessions', { status: 'SESSION_STATUS_ACTIVE' });
		return (sessions ?? []).find((s: { resourceName: string; front: string }) => s.resourceName === 'e2e-web-ssh' && s.front === 'ssh')?.id;
	};
	let cmd = run();
	try {
		// the owner is refused until the gateway has the binding: start again
		await expect
			.poll(
				async () => {
					if (cmd.done) cmd = run();
					return active();
				},
				{ timeout: 20_000 }
			)
			.toBeTruthy();
		const id = (await active())!;

		await page.goto(`/sessions/${id}`);
		await page.getByRole('button', { name: en.sessions_watch }).click();
		await expect(page).toHaveURL(`/sessions/${id}/live`);
		// the observer sees only what is written after it attached: write until it shows
		await expect(async () => {
			cmd.p.stdin.write('echo werbot-live-marker\n');
			await expect(page.getByTestId('xterm')).toContainText('werbot-live-marker', { timeout: 1_000 });
		}).toPass({ timeout: 15_000 });

		// the live view is read-only: typing into it reaches no one
		await page.getByTestId('xterm').click();
		await page.keyboard.type('echo werbot-observer-input\n');
		await page.waitForTimeout(500);
		expect(cmd.stdout).not.toContain('werbot-observer-input');

		await page.getByRole('button', { name: en.live_terminate, exact: true }).click();
		await page.getByRole('dialog').getByRole('textbox').fill('e2e');
		await page.getByRole('dialog').getByRole('button', { name: en.live_terminate_confirm }).click();
		await expect.poll(() => cmd.done, { timeout: 5_000 }).toBe(true);
		await expect(page.getByText(en.terminal_closed)).toBeVisible();
		await expect
			.poll(async () => (await rpc(page, 'werbot.v1.SessionService/GetSession', { id })).session.endReason, { timeout: 10_000 })
			.toBe('terminated');

		await page.goto('/audit?action=session');
		await expect(page.getByText('session.observe').first()).toBeVisible();
		await expect(page.getByText('session.terminate').first()).toBeVisible();
	} finally {
		cmd.p.kill();
	}
});
