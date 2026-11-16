import { expect, type Page } from '@playwright/test';
import en from '../../messages/en.json' with { type: 'json' };

export const OWNER = {
	email: 'owner@example.com',
	login: 'owner',
	name: 'Owner',
	password: 'correct horse battery',
	org: 'Acme'
};

export async function loginAsOwner(page: Page): Promise<void> {
	await page.goto('/login');
	await page.getByLabel(en.field_email, { exact: true }).fill(OWNER.email);
	await page.getByLabel(en.field_password, { exact: true }).fill(OWNER.password);
	await page.getByRole('button', { name: en.auth_login_submit }).click();
	await expect(page).toHaveURL('/');
}
