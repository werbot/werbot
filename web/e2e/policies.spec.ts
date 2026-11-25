import { expect, test } from '@playwright/test';
import en from '../messages/en.json' with { type: 'json' };
import { choose } from './support/select';
import { loginAsOwner } from './support/auth';
import { rpc } from './support/rpc';

const svc = 'werbot.v1.PolicyService/';

test('command rule, override, test box, custom policy and output masking', async ({ page }) => {
	test.setTimeout(90_000);
	await loginAsOwner(page);
	// the embedded gateway registers its "local" pool shortly after the server starts
	const pool = async () =>
		((await rpc(page, 'werbot.v1.GatewayService/ListGateways', {})).gateways ?? []).find((g: { name: string }) => g.name === 'local');
	await expect.poll(pool, { timeout: 30_000 }).toBeTruthy();
	const local = await pool();
	const { resource } = await rpc(page, 'werbot.v1.ResourceService/CreateResource', {
		name: 'e2e-cmd',
		type: 'ssh',
		gatewayId: local.id,
		config: { '@type': 'type.googleapis.com/werbot.connector.ssh.v1.Config', host: '127.0.0.1', port: 2022, login: 'e2e' }
	});
	const { user } = await rpc(page, 'werbot.v1.AuthService/GetSession', {});
	await rpc(page, 'werbot.v1.AccessService/CreateBinding', { subjectType: 'user', subjectId: user.id, resourceId: resource.id });

	try {
		// a deny rule for destructive commands on e2e-cmd
		await page.goto('/policies/commands');
		await page.getByRole('button', { name: en.policies_commands_new }).click();
		const drawer = page.getByRole('dialog', { name: en.policies_commands_new });
		await drawer.getByLabel(en.policies_commands_name).fill('no-destructive');
		await drawer.getByText(en.policies_commands_all_users, { exact: true }).click();
		await choose(drawer.getByLabel(en.approvals_add_resource), 'e2e-cmd');
		await drawer.getByText(en.class_destructive, { exact: true }).click();
		await drawer.getByLabel(en.policies_commands_message).fill('Destructive commands are blocked');
		await drawer.getByRole('button', { name: en.policies_commands_create }).click();
		await expect(drawer).toHaveCount(0);
		await expect(page.getByRole('button', { name: 'no-destructive', exact: true })).toBeVisible();

		// an override classifies a script as write
		await page.getByLabel(en.policies_class_rules_pattern, { exact: true }).fill('./deploy.sh');
		const overrides = page.locator('section').filter({ has: page.getByRole('heading', { name: en.policies_class_rules_title }) });
		await overrides.getByText(en.class_write, { exact: true }).click();
		await overrides.getByRole('button', { name: en.policies_class_rules_add }).click();
		await expect(overrides.getByRole('cell', { name: './deploy.sh' })).toBeVisible();

		// the test box
		const probe = page.locator('section').filter({ has: page.getByRole('heading', { name: en.policies_test_title }) });
		await choose(probe.getByLabel(en.policies_test_user), 'Owner (owner)');
		await choose(probe.getByLabel(en.policies_test_resource), 'e2e-cmd');
		await probe.getByLabel(en.policies_test_command).fill('rm -rf /');
		await probe.getByRole('button', { name: en.policies_test_run }).click();
		const result = probe.getByTestId('test-result');
		await expect(result.getByText(en.policies_test_denied)).toBeVisible();
		await expect(result.getByText(en.class_destructive, { exact: true })).toBeVisible();
		await expect(probe.getByText('Destructive commands are blocked')).toBeVisible();
		await expect(result.getByText('no-destructive', { exact: true })).toBeVisible();
		await probe.getByLabel(en.policies_test_command).fill('./deploy.sh');
		await probe.getByRole('button', { name: en.policies_test_run }).click();
		await expect(result.getByText(en.policies_test_allowed)).toBeVisible();
		await expect(result.getByText(en.class_write)).toBeVisible();

		// custom Cedar: a broken text toasts the server error with its position and stays in the editor
		await page.goto('/policies/custom');
		await page.getByRole('button', { name: en.policies_custom_new }).first().click();
		const editor = page.getByRole('dialog', { name: en.policies_custom_new });
		await editor.getByLabel(en.policies_custom_name).fill('e2e-forbid');
		await editor.getByLabel(en.policies_custom_cedar).fill('permit (');
		await editor.getByRole('button', { name: en.settings_save }).click();
		await expect(page.getByText(/\d+:\d+/)).toBeVisible();
		await expect(editor.getByLabel(en.policies_custom_cedar)).toHaveValue('permit (');
		await editor.getByLabel(en.policies_custom_cedar).fill('forbid (principal, action, resource) when { context.hour == 99 };');
		await editor.getByRole('button', { name: en.settings_save }).click();
		await expect(page.getByText(en.policies_custom_saved)).toBeVisible();
		const sw = page.getByRole('switch', { name: new RegExp('e2e-forbid') });
		await expect(sw).toBeChecked();
		await sw.locator('xpath=ancestor::label').click();
		await expect(sw).not.toBeChecked();

		// output masking persists
		await page.goto('/settings');
		await choose(page.getByLabel(en.settings_mask_output), en.settings_mask_output_all);
		await page.getByRole('button', { name: en.settings_save }).last().click();
		await expect(page.getByText(en.settings_saved)).toBeVisible();
		await page.reload();
		await expect(page.getByLabel(en.settings_mask_output)).toHaveText(en.settings_mask_output_all);
	} finally {
		await rpc(page, 'werbot.v1.SettingsService/UpdateSettings', { settings: { maskOutput: 'agents' }, updateMask: 'maskOutput' });
		for (const p of (await rpc(page, svc + 'ListCustomPolicies', {})).policies ?? []) await rpc(page, svc + 'DeleteCustomPolicy', { id: p.id });
		for (const r of (await rpc(page, svc + 'ListClassRules', {})).rules ?? []) await rpc(page, svc + 'DeleteClassRule', { id: r.id });
		for (const r of (await rpc(page, svc + 'ListCommandRules', {})).rules ?? []) await rpc(page, svc + 'DeleteCommandRule', { id: r.id });
		await rpc(page, 'werbot.v1.ResourceService/DeleteResource', { id: resource.id });
	}
});
