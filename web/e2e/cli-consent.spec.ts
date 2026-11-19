import { createHash, randomBytes } from 'node:crypto';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, test, type Page } from '@playwright/test';
import en from '../messages/en.json' with { type: 'json' };
import { loginAsOwner, loginWithNext } from './support/auth';

const HOST = 'pw-host';

// trap stands in for the CLI's loopback listener and resolves with the callback query.
async function trap() {
	let hit!: (q: URLSearchParams) => void;
	const query = new Promise<URLSearchParams>((r) => (hit = r));
	const srv = createServer((req, res) => {
		hit(new URL(req.url ?? '/', 'http://127.0.0.1').searchParams);
		res.end('ok');
	});
	await new Promise<void>((r) => srv.listen(0, '127.0.0.1', r));
	const { port } = srv.address() as AddressInfo;
	return { uri: `http://127.0.0.1:${port}/callback`, query, close: () => srv.close() };
}

function authorizeURL(redirectURI: string): string {
	const challenge = createHash('sha256').update(randomBytes(32).toString('base64url')).digest('base64url');
	return (
		'/oauth/authorize?' +
		new URLSearchParams({
			client_id: 'werbot-cli',
			response_type: 'code',
			scope: 'openid offline_access',
			redirect_uri: redirectURI,
			state: 'pw-state',
			code_challenge: challenge,
			code_challenge_method: 'S256',
			werbot_hostname: HOST
		})
	);
}

// the consent card names the CLI's host
const card = (page: Page) => page.getByText(HOST, { exact: true }).first();

test('loopback sign-in: login with next, approve hands the CLI a code', async ({ page }) => {
	const cli = await trap();
	try {
		await page.goto(authorizeURL(cli.uri));
		await expect(page).toHaveURL(/\/login\?next=%2Fcli%2Fauthorize%3Fid%3D[0-9a-f-]{36}$/);
		await loginWithNext(page);
		await expect(page).toHaveURL(/\/cli\/authorize\?id=[0-9a-f-]{36}$/);
		await expect(card(page)).toBeVisible();
		await page.getByRole('button', { name: en.cli_authorize_approve }).click();
		const q = await cli.query;
		expect(q.get('code')).toBeTruthy();
		expect(q.get('state')).toBe('pw-state');
	} finally {
		cli.close();
	}
});

test('loopback sign-in: deny hands the CLI interaction_required', async ({ page }) => {
	const cli = await trap();
	try {
		await loginAsOwner(page);
		await page.goto(authorizeURL(cli.uri));
		await expect(card(page)).toBeVisible();
		await page.getByRole('button', { name: en.cli_authorize_deny }).click();
		const q = await cli.query;
		expect(q.get('error')).toBe('interaction_required');
		expect(q.get('code')).toBeNull();
	} finally {
		cli.close();
	}
});

test('device sign-in, then the session shows in the profile and is revoked', async ({ page, request }) => {
	const res = await request.post('/oauth/device_authorization', {
		form: { client_id: 'werbot-cli', scope: 'openid offline_access', werbot_hostname: HOST }
	});
	expect(res.ok()).toBe(true);
	const { device_code: deviceCode, user_code: userCode } = await res.json();

	await loginAsOwner(page);
	await page.goto(`/device?user_code=${encodeURIComponent(userCode)}`);
	await expect(card(page)).toBeVisible();
	await page.getByRole('button', { name: en.device_approve }).click();
	await expect(page.getByText(en.device_done)).toBeVisible();

	const poll = () =>
		request
			.post('/oauth/token', {
				form: { grant_type: 'urn:ietf:params:oauth:grant-type:device_code', client_id: 'werbot-cli', device_code: deviceCode }
			})
			.then((r) => r.json());
	await expect.poll(async () => (await poll()).access_token ?? '', { timeout: 15_000 }).not.toBe('');

	await page.goto('/profile/cli-sessions');
	const row = page.getByRole('row', { name: new RegExp(HOST) });
	await expect(row).toBeVisible();
	await row.getByRole('button', { name: en.common_more }).click();
	await page.getByRole('menuitem', { name: en.cli_sessions_revoke }).click();
	const confirm = page.getByRole('dialog', { name: en.cli_sessions_revoke });
	await expect(confirm.getByText(en.cli_sessions_revoke_confirm)).toBeVisible();
	await confirm.getByRole('button', { name: en.cli_sessions_revoke }).click();
	await expect(page.getByText(en.cli_sessions_empty)).toBeVisible();
});

test('device page: an unknown code toasts and keeps the form', async ({ page }) => {
	await loginAsOwner(page);
	await page.goto('/device');
	await page.getByLabel(en.device_code_label).fill('BCDF-GHJK');
	await page.getByRole('button', { name: en.device_lookup }).click();
	await expect(page.getByText(en.device_not_found)).toBeVisible();
	await expect(page.getByLabel(en.device_code_label)).toBeVisible();
});
