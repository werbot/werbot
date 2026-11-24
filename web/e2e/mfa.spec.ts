import { randomUUID } from 'node:crypto';
import { expect, test, type Page } from '@playwright/test';
import en from '../messages/en.json' with { type: 'json' };
import { BOB, loginAsOwner, loginWithNext } from './support/auth';
import { rpc } from './support/rpc';
import { totp } from './support/totp';
import { addVirtualAuthenticator } from './support/webauthn';

const FACTORS = 'werbot.v1.MFAService';

// Removes every factor the signed-in user has, so later specs sign in with a password alone.
async function removeFactors(page: Page) {
	const { factors = [] } = await rpc(page, `${FACTORS}/ListFactors`, {});
	for (const f of factors) await rpc(page, `${FACTORS}/DeleteFactor`, { id: f.id });
}

async function logout(page: Page) {
	await rpc(page, 'werbot.v1.AuthService/Logout', {});
	await page.goto('/login');
}

test.describe.serial('mfa', () => {
	test('a member adds a passkey, an authenticator and recovery codes, then signs in with each factor', async ({ page }) => {
		await addVirtualAuthenticator(page);
		await page.goto('/login');
		await loginWithNext(page, BOB);
		await expect(page).toHaveURL('/');
		await page.goto('/profile/mfa');

		// the first factor needs no confirmation
		await page.getByRole('button', { name: en.mfa_passkey_add }).first().click();
		const passkey = page.getByRole('dialog', { name: en.mfa_passkey_add });
		await passkey.getByLabel(en.mfa_passkey_name).fill('e2e passkey');
		await passkey.getByRole('button', { name: en.mfa_passkey_add }).click();
		await expect(page.getByRole('cell', { name: 'e2e passkey' })).toBeVisible();

		// a second factor needs a fresh one: the step-up dialog opens, the passkey confirms, the request repeats
		await page.getByRole('button', { name: en.mfa_totp_add }).click();
		const stepUp = page.getByRole('dialog', { name: en.mfa_stepup_title });
		await stepUp.getByRole('button', { name: en.auth_mfa_passkey }).click();
		const totpDialog = page.getByRole('dialog', { name: en.mfa_totp_title });
		await expect(totpDialog.getByRole('img', { name: en.mfa_totp_qr })).toBeVisible();
		const secret = (await totpDialog.locator('code.secret').textContent())!.trim();
		await totpDialog.getByLabel(en.auth_mfa_totp_code).fill(totp(secret));
		await totpDialog.getByRole('button', { name: en.mfa_totp_confirm }).click();
		await expect(page.getByRole('cell', { name: en.mfa_kind_totp }).first()).toBeVisible();

		await page.getByRole('button', { name: en.mfa_recovery_generate }).click();
		const recovery = page.getByRole('dialog', { name: en.mfa_recovery_title });
		await expect(recovery.locator('code')).toHaveCount(10);
		const codes = await recovery.locator('code').allTextContents();
		await recovery.getByRole('button', { name: en.mfa_recovery_saved }).click();

		// sign in again: password, then the MFA step with the passkey
		await logout(page);
		await loginWithNext(page, BOB);
		await expect(page.getByRole('heading', { name: en.auth_mfa_title })).toBeVisible();
		await page.getByRole('button', { name: en.auth_mfa_passkey }).click();
		await expect(page).toHaveURL('/');

		// and with a recovery code
		await logout(page);
		await loginWithNext(page, BOB);
		await page.getByRole('button', { name: en.auth_mfa_use_recovery }).click();
		await page.getByLabel(en.auth_mfa_recovery_code).fill(codes[0]);
		await page.getByRole('button', { name: en.auth_mfa_verify }).click();
		await expect(page).toHaveURL('/');

		// a recovery login is a fresh confirmation, so the factors can go
		await removeFactors(page);
		await page.goto('/profile/mfa');
		await expect(page.getByText(en.mfa_empty)).toBeVisible();
	});

	test('an admin action asks for a fresh factor, then an MFA rule is saved', async ({ page }) => {
		await addVirtualAuthenticator(page);
		await loginAsOwner(page);
		await page.goto('/profile/mfa');
		await page.getByRole('button', { name: en.mfa_passkey_add }).first().click();
		await page.getByRole('dialog', { name: en.mfa_passkey_add }).getByRole('button', { name: en.mfa_passkey_add }).click();
		await expect(page.getByRole('cell', { name: en.mfa_passkey_default_name })).toBeVisible();

		await page.goto('/policies/mfa');
		await page.getByRole('button', { name: en.mfa_rules_new }).first().click();
		const drawer = page.getByRole('dialog', { name: en.mfa_rules_new });
		await drawer.getByLabel(en.mfa_rules_name, { exact: true }).fill('e2e-mfa-rule');
		await drawer.getByText(en.policies_commands_all_users, { exact: true }).click();
		await drawer.getByRole('tab', { name: en.policies_commands_target_labels }).click();
		await drawer.getByRole('button', { name: en.resources_label_add }).click();
		await drawer.getByLabel(en.resources_label_key).fill('env');
		await drawer.getByLabel(en.resources_label_value).fill('prod');
		await drawer.getByRole('button', { name: en.mfa_rules_create }).click();
		await page.getByRole('dialog', { name: en.mfa_stepup_title }).getByRole('button', { name: en.auth_mfa_passkey }).click();
		await expect(page.getByRole('button', { name: 'e2e-mfa-rule' })).toBeVisible();

		const { rules } = await rpc(page, 'werbot.v1.PolicyService/ListMFARules', {});
		for (const r of rules) await rpc(page, 'werbot.v1.PolicyService/DeleteMFARule', { id: r.id });
		await removeFactors(page);
	});

	test('the approve page says a request is missing', async ({ page }) => {
		await loginAsOwner(page);
		await page.goto(`/mfa/${randomUUID()}`);
		await expect(page.getByText(en.mfa_challenge_not_found)).toBeVisible();
	});

	test('an identity provider is saved without showing its secret and offered on the sign-in page', async ({ page, browser, baseURL }) => {
		await loginAsOwner(page);
		await page.goto('/settings/sso');
		await page.getByRole('button', { name: en.sso_new }).first().click();
		const drawer = page.getByRole('dialog', { name: en.sso_new });
		await drawer.getByLabel(en.sso_name, { exact: true }).fill('E2E IdP');
		await drawer.getByLabel(en.sso_issuer).fill('http://127.0.0.1:9/realms/e2e');
		await drawer.getByLabel(en.sso_client_id).fill('werbot');
		await drawer.getByLabel(en.sso_client_secret, { exact: true }).fill('very-secret-value');
		await drawer.getByRole('button', { name: en.sso_mapping_add }).click();
		await drawer.getByLabel(en.sso_mapping_regex).fill('^/admins$');
		await drawer.getByRole('tab', { name: en.sso_mapping_role }).click();
		await drawer.getByLabel(en.sso_mapping_role, { exact: true }).selectOption('admin');
		await drawer.getByRole('button', { name: en.sso_create }).click();
		await expect(page.getByText(en.common_saved)).toBeVisible();

		// the drawer stays on the saved provider: the redirect URI is there, the secret is not
		const saved = page.getByRole('dialog', { name: 'E2E IdP' });
		await expect(saved.getByLabel(en.sso_redirect_uri)).toHaveValue(`${baseURL}/api/oidc/callback`);
		await expect(saved.getByLabel(en.sso_client_secret, { exact: true })).toHaveValue('');
		await expect(page.getByText('very-secret-value')).toHaveCount(0);

		const anon = await browser.newContext({ baseURL });
		const login = await anon.newPage();
		await login.goto('/login?error=oidc');
		await expect(login.getByText(en.auth_oidc_failed)).toBeVisible();
		await expect(login.getByRole('button', { name: en.auth_sso_with.replace('{name}', 'E2E IdP') })).toBeVisible();
		await anon.close();

		const { identityProviders } = await rpc(page, 'werbot.v1.IdentityProviderService/ListIdentityProviders', {});
		for (const p of identityProviders) await rpc(page, 'werbot.v1.IdentityProviderService/DeleteIdentityProvider', { id: p.id });
	});
});
