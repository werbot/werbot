import { expect, test } from '@playwright/test';
import en from '../messages/en.json' with { type: 'json' };
import { loginAsOwner } from './support/auth';
import { rpc } from './support/rpc';

const svc = 'werbot.v1.PolicyService/';

test('path rule template, test box paths, terminal rule, allowlist preset', async ({ page }) => {
	test.setTimeout(90_000);
	await loginAsOwner(page);
	// the embedded gateway registers its "local" pool shortly after the server starts
	const pool = async () =>
		((await rpc(page, 'werbot.v1.GatewayService/ListGateways', {})).gateways ?? []).find((g: { name: string }) => g.name === 'local');
	await expect.poll(pool, { timeout: 30_000 }).toBeTruthy();
	const local = await pool();
	const { resource } = await rpc(page, 'werbot.v1.ResourceService/CreateResource', {
		name: 'e2e-paths',
		type: 'ssh',
		gatewayId: local.id,
		config: { '@type': 'type.googleapis.com/werbot.connector.ssh.v1.Config', host: '127.0.0.1', port: 2022, login: 'e2e' }
	});
	const { user } = await rpc(page, 'werbot.v1.AuthService/GetSession', {});
	await rpc(page, 'werbot.v1.AccessService/CreateBinding', { subjectType: 'user', subjectId: user.id, resourceId: resource.id });

	try {
		// a template fills folders, access and effect
		await page.goto('/policies/paths');
		await page.getByRole('button', { name: en.policies_paths_new }).first().click();
		const drawer = page.getByRole('dialog', { name: en.policies_paths_new });
		await drawer.getByRole('button', { name: en.policies_paths_template_work_folder }).click();
		await expect(drawer.getByLabel(en.policies_paths_paths, { exact: true })).toHaveValue('~/app/**');
		await expect(drawer.getByText(en.policies_paths_home_hint)).toBeVisible();
		await drawer.getByLabel(en.policies_paths_name).fill('work');
		await drawer.getByText(en.policies_commands_all_users, { exact: true }).click();
		await drawer.getByLabel(en.approvals_add_resource).selectOption({ label: 'e2e-paths' });
		await drawer.getByRole('button', { name: en.policies_paths_create }).click();
		await expect(drawer).toHaveCount(0);
		const row = page.getByRole('row').filter({ has: page.getByRole('button', { name: 'work', exact: true }) });
		await expect(row.getByText('~/app/**')).toBeVisible();
		await expect(row.getByText(en.policies_paths_effect_only)).toBeVisible();

		// the test box shows the paths and the path rule outcome
		await page.goto('/policies/commands');
		const probe = page.locator('section').filter({ has: page.getByRole('heading', { name: en.policies_test_title }) });
		await probe.getByLabel(en.policies_test_user).selectOption({ label: 'Owner (owner)' });
		await probe.getByLabel(en.policies_test_resource).selectOption({ label: 'e2e-paths' });
		await probe.getByLabel(en.policies_test_command).fill('cat /etc/hosts');
		await probe.getByRole('button', { name: en.policies_test_run }).click();
		const result = probe.getByTestId('test-result');
		await expect(result.getByText(en.policies_test_denied)).toBeVisible();
		await expect(result.getByText('/etc/hosts')).toBeVisible();
		await expect(result.getByText('work', { exact: true })).toBeVisible();
		await probe.getByLabel(en.policies_test_cwd).fill('~/app');
		await probe.getByLabel(en.policies_test_command).fill('cat a');
		await probe.getByRole('button', { name: en.policies_test_run }).click();
		await expect(result.getByText(en.policies_test_allowed)).toBeVisible();
		await expect(result.getByText('~/app/a')).toBeVisible();

		// a glob that is not absolute or home-relative toasts the server error
		await page.goto('/policies/paths');
		await page.getByRole('button', { name: en.policies_paths_new }).first().click();
		const bad = page.getByRole('dialog', { name: en.policies_paths_new });
		await bad.getByLabel(en.policies_paths_name).fill('bad');
		await bad.getByText(en.policies_commands_all_users, { exact: true }).click();
		await bad.getByLabel(en.approvals_add_resource).selectOption({ label: 'e2e-paths' });
		await bad.getByLabel(en.policies_paths_paths, { exact: true }).fill('app/**');
		await bad.getByRole('button', { name: en.policies_paths_create }).click();
		await expect(page.getByText(/invalid path glob/)).toBeVisible();
		await bad.getByRole('button', { name: en.common_cancel }).click();

		// a terminal rule
		await page.goto('/policies/terminal');
		await page.getByRole('button', { name: en.policies_terminal_new }).first().click();
		const term = page.getByRole('dialog', { name: en.policies_terminal_new });
		await term.getByLabel(en.policies_terminal_name).fill('console-only');
		await term.getByText(en.policies_commands_all_users, { exact: true }).click();
		await term.getByLabel(en.approvals_add_resource).selectOption({ label: 'e2e-paths' });
		await term.getByRole('button', { name: en.policies_terminal_create }).click();
		await expect(term).toHaveCount(0);
		const trow = page.getByRole('row').filter({ hasText: 'console-only' });
		await expect(trow.getByText(en.policies_terminal_mode_console, { exact: true })).toBeVisible();

		// the allowlist preset opens the form with opaque and deny
		await page.goto('/policies/commands');
		await page.getByRole('button', { name: en.policies_commands_preset_allowlist }).click();
		const preset = page.getByRole('dialog');
		await expect(preset.getByLabel(en.policies_commands_name)).toHaveValue('only-allowed-commands');
		await expect(preset.getByLabel(en.class_opaque)).toBeChecked();
		await expect(preset.getByRole('tab', { name: en.policies_commands_effect_deny })).toHaveAttribute('aria-selected', 'true');
	} finally {
		for (const r of (await rpc(page, svc + 'ListPathRules', {})).rules ?? []) await rpc(page, svc + 'DeletePathRule', { id: r.id });
		for (const r of (await rpc(page, svc + 'ListTerminalRules', {})).rules ?? []) await rpc(page, svc + 'DeleteTerminalRule', { id: r.id });
		await rpc(page, 'werbot.v1.ResourceService/DeleteResource', { id: resource.id });
	}
});
