import { expect, test } from '@playwright/test';

// `request` has its own cookie jar: the session Setup creates does not redirect `page` away from /login.
test('login page renders on a fresh install', async ({ page, request }) => {
	const state = await request.post('/werbot.v1.AuthService/GetSetupState', { data: {} });
	expect(state.ok()).toBe(true);
	if ((await state.json()).needsSetup) {
		const setup = await request.post('/werbot.v1.AuthService/Setup', {
			data: { email: 'smoke@example.com', login: 'smoke', name: 'Smoke', password: 'smoke-password-42', organizationName: 'Smoke' }
		});
		expect(setup.ok()).toBe(true);
	}
	await page.goto('/login');
	await expect(page.locator('input[type="email"]')).toBeVisible();
	await expect(page.locator('input[type="password"]')).toBeVisible();
	await expect(page.locator('button[type="submit"]')).toBeVisible();
});
