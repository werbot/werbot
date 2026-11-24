import { expect, test, type Cookie, type Page } from '@playwright/test';
import en from '../messages/en.json' with { type: 'json' };
import { OWNER } from '../e2e/support/auth';
import { rpc } from '../e2e/support/rpc';
import { shot } from '../e2e/support/screens';
import { addVirtualAuthenticator } from '../e2e/support/webauthn';

const ISSUER = 'http://127.0.0.1:18180/realms/werbot'; // keycloak.yaml, realm werbot
const IDP = 'Keycloak red-team';
const SHOTS = 'sdd/stage16-beta/screens/t4';

// Signs in through the identity provider as the realm's sso-alice.
async function signInWithSSO(page: Page) {
	await page.goto('/login');
	await page.getByRole('button', { name: en.auth_sso_with.replace('{name}', IDP) }).click();
	await page.locator('#username').fill('sso-alice');
	await page.locator('#password').fill('sso-alice-password');
	await page.locator('#kc-login').click();
	await expect(page).toHaveURL('/');
}

async function events(page: Page, action: string): Promise<{ actorLogin: string; data?: Record<string, unknown> }[]> {
	return (await rpc(page, 'werbot.v1.AuditService/ListEvents', { action, pageSize: 200 })).events ?? [];
}

test('keycloak oidc login, passkey and step-up MFA on an admin action', async ({ page }) => {
	test.setTimeout(120_000); // the first Keycloak login in start-dev mode is slow
	// the owner: installation and the identity provider, both by RPC
	await rpc(page, 'werbot.v1.AuthService/Setup', {
		email: OWNER.email,
		login: OWNER.login,
		name: OWNER.name,
		password: OWNER.password,
		organizationName: OWNER.org
	});
	await rpc(page, 'werbot.v1.AuthService/Login', { email: OWNER.email, password: OWNER.password });
	const { identityProvider } = await rpc(page, 'werbot.v1.IdentityProviderService/CreateIdentityProvider', {
		name: IDP,
		issuer: ISSUER,
		clientId: 'werbot',
		clientSecret: 'werbot-test-secret',
		scopes: ['openid', 'email', 'profile'], // the realm's client mapper puts groups in the ID token
		claimsMapping: [{ claim: 'groups', regex: '^/admins$', role: 'admin' }],
		allowSignup: true,
		enabled: true
	});
	const owner = await page.context().cookies();
	let lockId = '';
	let alice: Cookie[] = [];
	try {
		// session A: sso-alice signs in, the group claim makes her an admin
		await page.context().clearCookies();
		await addVirtualAuthenticator(page);
		await signInWithSSO(page);
		await shot(page, `${SHOTS}-1-signed-in.png`);
		const session = await rpc(page, 'werbot.v1.AuthService/GetSession', {});
		expect(session.role).toBe('ROLE_ADMIN');
		expect(session.user.login).not.toBe(OWNER.login);
		alice = await page.context().cookies();

		// session B: the same user signs in again and enrols a passkey
		await page.context().clearCookies();
		await signInWithSSO(page);
		await page.goto('/profile/mfa');
		await page.getByRole('button', { name: en.mfa_passkey_add }).first().click();
		await page.getByRole('dialog', { name: en.mfa_passkey_add }).getByRole('button', { name: en.mfa_passkey_add }).click();
		await expect(page.getByRole('cell', { name: en.mfa_passkey_default_name })).toBeVisible();
		await shot(page, `${SHOTS}-2-passkey.png`);
		const { factors } = await rpc(page, 'werbot.v1.MFAService/ListFactors', {});
		expect(factors.filter((f: { kind: string }) => f.kind === 'webauthn')).toHaveLength(1);

		// back in session A: it holds no fresh factor, so the admin action asks for one
		await page.context().clearCookies();
		await page.context().addCookies(alice);
		await page.goto('/locks');
		await page.getByRole('button', { name: en.locks_new }).first().click();
		const form = page.getByRole('dialog', { name: en.locks_new });
		await form.getByLabel(en.locks_target_type).selectOption({ label: en.locks_type_all_agents });
		await form.getByLabel(en.locks_reason).fill('redteam-sso');
		await form.getByRole('button', { name: en.locks_new }).click();
		const stepUp = page.getByRole('dialog', { name: en.auth_mfa_title });
		await expect(stepUp).toBeVisible();
		await shot(page, `${SHOTS}-3-step-up.png`);
		const before = (await rpc(page, 'werbot.v1.LockService/ListLocks', {})).locks ?? [];
		expect(before).toHaveLength(0); // nothing changed before the factor
		await stepUp.getByRole('button', { name: en.auth_mfa_passkey }).click();
		await expect(page.getByRole('row').filter({ hasText: 'redteam-sso' })).toBeVisible();
		await shot(page, `${SHOTS}-4-lock-created.png`);
		lockId = (await rpc(page, 'werbot.v1.LockService/ListLocks', {})).locks[0].id;

		// the journal, read by the owner
		await page.context().clearCookies();
		await page.context().addCookies(owner);
		const logins = await events(page, 'auth.login');
		expect(logins.some((e) => e.data?.method === 'oidc')).toBe(true);
		expect((await events(page, 'mfa.factor.add')).some((e) => e.data?.kind === 'webauthn')).toBe(true);
		expect((await events(page, 'lock.create')).length).toBeGreaterThan(0);
	} finally {
		// delete what the test made: the lock, alice's factors, the provider
		await page.context().clearCookies();
		await page.context().addCookies(owner);
		if (lockId) await rpc(page, 'werbot.v1.LockService/DeleteLock', { id: lockId }).catch(() => {});
		await rpc(page, 'werbot.v1.IdentityProviderService/DeleteIdentityProvider', { id: identityProvider.id }).catch(() => {});
		if (alice.length) {
			await page.context().clearCookies();
			await page.context().addCookies(alice);
			const { factors = [] } = await rpc(page, 'werbot.v1.MFAService/ListFactors', {}).catch(() => ({}));
			for (const f of factors) await rpc(page, 'werbot.v1.MFAService/DeleteFactor', { id: f.id }).catch(() => {});
		}
	}
});
