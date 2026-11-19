import { expect, test } from '@playwright/test';
import en from '../messages/en.json' with { type: 'json' };
import { BOB as bob, OWNER, loginAsOwner } from './support/auth';
import { latestEmailLink } from './support/mail';

test.describe.serial('identity', () => {
	test('setup, logout, login', async ({ page }) => {
		await page.goto('/');
		await expect(page).toHaveURL(/\/setup$/);
		await page.getByLabel(en.field_email, { exact: true }).fill(OWNER.email);
		await page.getByLabel(en.field_login, { exact: true }).fill(OWNER.login);
		await page.getByLabel(en.field_name, { exact: true }).fill(OWNER.name);
		await page.getByLabel(en.field_password, { exact: true }).fill(OWNER.password);
		await page.getByLabel(en.field_org_name).fill(OWNER.org);
		await page.getByRole('button', { name: en.auth_setup_submit }).click();
		await expect(page).toHaveURL('/');
		await expect(page.getByRole('link', { name: en.nav_organization })).toBeVisible();
		await expect(page.getByRole('heading', { name: OWNER.org })).toBeVisible();
		await expect(page.getByRole('heading', { name: en.dashboard_recent_sessions })).toBeVisible();
		await expect(page.getByText(en.dashboard_no_sessions)).toBeVisible();

		await page.getByRole('button', { name: en.nav_profile }).click();
		await page.getByRole('menuitem', { name: en.auth_logout }).click();
		await expect(page).toHaveURL(/\/login$/);
		await loginAsOwner(page);
	});

	test('invite and accept', async ({ page, browser }) => {
		await loginAsOwner(page);
		await page.goto('/members');
		await page.getByRole('button', { name: en.members_invite, exact: true }).click();
		await page.getByLabel(en.field_email, { exact: true }).fill(bob.email);
		await page.getByRole('button', { name: en.members_invite_submit }).click();
		await expect(page.getByText(en.members_invites_title)).toBeVisible();

		const link = await latestEmailLink(bob.email, 'invite');
		const ctx = await browser.newContext();
		const bobPage = await ctx.newPage();
		await bobPage.goto(link);
		await bobPage.getByLabel(en.field_login, { exact: true }).fill(bob.login);
		await bobPage.getByLabel(en.field_name, { exact: true }).fill(bob.name);
		await bobPage.getByLabel(en.field_password, { exact: true }).fill(bob.password);
		await bobPage.getByRole('button', { name: en.invite_accept_submit }).click();
		await expect(bobPage).toHaveURL('/');
		await expect(bobPage.getByRole('link', { name: en.nav_access })).toBeVisible();
		await expect(bobPage.getByRole('link', { name: en.nav_organization })).toBeHidden();
		await expect(bobPage.getByRole('heading', { name: en.dashboard_recent_sessions })).toBeVisible();
		await expect(bobPage.getByRole('heading', { name: en.dashboard_resource_health })).toBeHidden();
		await ctx.close();
	});

	test('create team and logout', async ({ page }) => {
		await loginAsOwner(page);
		await page.goto('/teams');
		await page.getByRole('button', { name: en.teams_new }).click();
		await page.getByLabel(en.teams_name).fill('ops');
		await page.getByRole('button', { name: en.teams_create_submit }).click();
		await expect(page.getByRole('button', { name: 'ops', exact: true })).toBeVisible();
		await page.getByRole('button', { name: 'ops', exact: true }).click();

		await page.getByLabel(en.teams_add_member).selectOption({ label: bob.name });
		await expect(page.locator('.miso-chip', { hasText: bob.name })).toBeVisible();
		await page.keyboard.press('Escape');

		await page.getByRole('button', { name: en.nav_profile }).click();
		await page.getByRole('menuitem', { name: en.auth_logout }).click();
		await expect(page).toHaveURL(/\/login$/);
		await page.goto('/teams');
		await expect(page).toHaveURL(/\/login\?next=%2Fteams$/);
	});
});
