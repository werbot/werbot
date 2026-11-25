import { expect, type Cookie, type Page } from '@playwright/test';
import en from '../../messages/en.json' with { type: 'json' };

export const OWNER = {
	email: 'owner@example.com',
	login: 'owner',
	name: 'Owner',
	password: 'correct horse battery',
	org: 'Acme'
};

// a member the identity project invites
export const BOB = { email: 'bob@example.com', login: 'bob', name: 'Bob', password: 'another long password' };

// The owner's session cookies, reused across tests: the server allows 10
// logins a minute per IP and email, fewer than the suite performs.
let ownerCookies: Cookie[] | undefined;

export async function loginAsOwner(page: Page): Promise<void> {
	if (ownerCookies) {
		await page.context().addCookies(ownerCookies);
		const res = await page.request.post('/werbot.v1.AuthService/GetSession', { data: {} });
		if (res.ok()) {
			await page.goto('/');
			await expect(page).toHaveURL('/');
			return;
		}
		await page.context().clearCookies(); // the session ended (logout, password change)
	}
	await page.goto('/login');
	await loginWithNext(page);
	await expect(page).toHaveURL('/');
	ownerCookies = await page.context().cookies();
}

// Fills the login form already on screen; the caller checks where `next` leads.
export async function loginWithNext(page: Page, user: { email: string; password: string } = OWNER): Promise<void> {
	await page.getByLabel(en.field_email, { exact: true }).fill(user.email);
	await page.getByLabel(en.field_password, { exact: true }).fill(user.password);
	await page.getByRole('button', { name: en.auth_login_submit }).click();
}
