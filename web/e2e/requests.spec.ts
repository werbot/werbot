import { expect, test } from '@playwright/test';
import en from '../messages/en.json' with { type: 'json' };
import { BOB, loginAsOwner, loginWithNext } from './support/auth';
import { rpc } from './support/rpc';

test('member requests access, owner approves, lock is created and removed', async ({ page, browser, baseURL }) => {
	test.setTimeout(90_000);
	await loginAsOwner(page);
	// its own resource: e2e-target belongs to sessions.spec, which runs later
	// the embedded gateway registers its "local" pool shortly after the server starts
	const pool = async () =>
		((await rpc(page, 'werbot.v1.GatewayService/ListGateways', {})).gateways ?? []).find((g: { name: string }) => g.name === 'local');
	await expect.poll(pool, { timeout: 30_000 }).toBeTruthy();
	const local = await pool();
	const { resource } = await rpc(page, 'werbot.v1.ResourceService/CreateResource', {
		name: 'e2e-jit',
		type: 'ssh',
		gatewayId: local.id,
		config: { '@type': 'type.googleapis.com/werbot.connector.ssh.v1.Config', host: '127.0.0.1', port: 2022, login: 'e2e' }
	});

	try {
		// owner: a manual policy "prod" on e2e-jit that every user may request, approved by owners and admins
		await page.goto('/policies/approvals');
		await page.getByRole('button', { name: en.approvals_new }).click();
		const policy = page.getByRole('dialog', { name: en.approvals_new });
		await policy.getByLabel(en.approvals_name).fill('prod');
		await policy.getByLabel(en.approvals_add_resource).selectOption({ label: 'e2e-jit' });
		await policy.getByText(en.approvals_all_users).click();
		await policy.getByRole('button', { name: en.approvals_create }).click();
		await expect(policy).toHaveCount(0);
		await expect(page.getByRole('button', { name: 'prod', exact: true })).toBeVisible();

		// bob (accepted in identity.spec) asks for 2h in his own browser context
		const bobContext = await browser.newContext({ baseURL });
		const bob = await bobContext.newPage();
		await bob.goto('/login');
		await loginWithNext(bob, BOB);
		await expect(bob).toHaveURL('/');
		await bob.goto('/requests');
		await bob.getByRole('button', { name: en.requests_new }).click();
		const form = bob.getByRole('dialog', { name: en.requests_new });
		await form.getByLabel(en.requests_resource).selectOption({ label: 'e2e-jit' });
		await form.getByLabel(en.requests_duration).fill('2h');
		await form.getByLabel(en.requests_reason).fill('deploy');
		await form.getByRole('button', { name: en.requests_submit }).click();
		await expect(bob).toHaveURL(/\/requests\?id=/);
		const bobDrawer = bob.getByRole('dialog', { name: 'e2e-jit' });
		await expect(bobDrawer.getByText(en.requests_status_pending)).toBeVisible();

		// owner: the "To approve" tab counts it; approve from the drawer
		await page.goto('/requests');
		const toApprove = page.getByRole('tab', { name: new RegExp(en.requests_tab_to_approve) });
		await expect(toApprove).toContainText('1');
		await toApprove.click();
		await page.getByRole('button', { name: 'e2e-jit' }).click();
		const drawer = page.getByRole('dialog', { name: 'e2e-jit' });
		await drawer.getByRole('button', { name: en.requests_approve }).click();
		await expect(page.getByText(en.requests_approved)).toBeVisible();
		await expect(drawer.getByText(en.requests_status_approved)).toBeVisible();
		await expect(toApprove).toContainText('0');

		// bob: the request is approved and the grant is active
		await bob.reload();
		await expect(bobDrawer.getByText(en.requests_status_approved)).toBeVisible();
		await bobDrawer.getByRole('button', { name: 'Close' }).click();
		await expect(bobDrawer).toHaveCount(0);
		await expect(bob).toHaveURL(/\/requests$/);
		await bob.getByRole('tab', { name: new RegExp(en.requests_grants_title) }).click();
		await expect(bob.getByRole('table').getByText('e2e-jit')).toBeVisible();
		await bobContext.close();

		// owner: lock bob, then delete the lock
		await page.goto('/locks');
		await page.getByRole('button', { name: en.locks_new }).click();
		const lock = page.getByRole('dialog', { name: en.locks_new });
		await lock.getByLabel(en.locks_target, { exact: true }).selectOption({ label: `${BOB.name} (${BOB.login})` });
		await lock.getByLabel(en.locks_reason).fill('incident');
		await lock.getByRole('button', { name: en.locks_new }).click();
		const row = page.getByRole('row').filter({ hasText: 'incident' });
		await expect(row).toContainText(`${BOB.name} (${BOB.login})`);
		await row.getByRole('button', { name: en.common_more }).click();
		await page.getByRole('menuitem', { name: en.locks_delete }).click();
		await page.getByRole('dialog', { name: en.locks_delete }).getByRole('button', { name: en.locks_delete }).click();
		await expect(page.getByText(en.locks_empty)).toBeVisible();

		// owner: strict lock mode persists
		await page.goto('/settings');
		await page.getByRole('tab', { name: en.settings_lock_mode_strict }).click();
		await page.getByRole('button', { name: en.settings_save }).nth(1).click();
		await expect(page.getByText(en.settings_saved)).toBeVisible();
		await page.reload();
		await expect(page.getByRole('tab', { name: en.settings_lock_mode_strict })).toHaveAttribute('aria-selected', 'true');
	} finally {
		// later specs share the database: best_effort again, no policy, no resource (its grant goes with it)
		await rpc(page, 'werbot.v1.SettingsService/UpdateSettings', { settings: { lockMode: 'best_effort' }, updateMask: 'lockMode' });
		const { approvalPolicies } = await rpc(page, 'werbot.v1.ApprovalPolicyService/ListApprovalPolicies', {});
		for (const p of approvalPolicies ?? []) await rpc(page, 'werbot.v1.ApprovalPolicyService/DeleteApprovalPolicy', { id: p.id });
		await rpc(page, 'werbot.v1.ResourceService/DeleteResource', { id: resource.id });
	}
});
