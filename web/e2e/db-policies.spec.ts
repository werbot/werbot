import { expect, test } from '@playwright/test';
import en from '../messages/en.json' with { type: 'json' };
import { choose } from './support/select';
import { loginAsOwner } from './support/auth';
import { rpc } from './support/rpc';
import { shot } from './support/screens';

const svc = 'werbot.v1.PolicyService/';
const agentSvc = 'werbot.v1.AgentService/';

test('database resource shows agent credential status and tests SQL rules', async ({ page }) => {
	test.setTimeout(90_000);
	await loginAsOwner(page);
	// the embedded gateway registers its "local" pool shortly after the server starts
	const pool = async () =>
		((await rpc(page, 'werbot.v1.GatewayService/ListGateways', {})).gateways ?? []).find((g: { name: string }) => g.name === 'local');
	await expect.poll(pool, { timeout: 30_000 }).toBeTruthy();
	const local = await pool();
	const { resource } = await rpc(page, 'werbot.v1.ResourceService/CreateResource', {
		name: 'e2e-db',
		type: 'postgres',
		gatewayId: local.id,
		config: {
			'@type': 'type.googleapis.com/werbot.connector.postgres.v1.Config',
			host: 'db.invalid',
			database: 'app',
			username: 'svc',
			sslmode: 'disable',
			agentUsername: 'werbot_agent'
		}
	});
	const { user } = await rpc(page, 'werbot.v1.AuthService/GetSession', {});
	await rpc(page, 'werbot.v1.AccessService/CreateBinding', { subjectType: 'user', subjectId: user.id, resourceId: resource.id });
	const { agent } = await rpc(page, agentSvc + 'CreateAgent', { name: 'e2e-db-agent' });

	try {
		// the agent credential: write-only, then checked by the gateway
		await page.goto(`/resources/${resource.id}`);
		await page.getByRole('tab', { name: en.resources_tab_credentials }).click();
		const block = page.getByRole('region', { name: en.resources_agent_credential });
		await expect(block.getByText(en.resources_agent_status_none, { exact: true })).toBeVisible();
		await block.getByLabel(en.cred_kind_password, { exact: true }).fill('agent-secret');
		await block.getByRole('button', { name: en.cred_save }).click();
		await expect(block.getByText(en.resources_agent_status_pending, { exact: true })).toBeVisible();
		await expect(block.getByText('GRANT SELECT ON ALL TABLES')).toBeVisible();
		await shot(page, 'sdd/stage18-sql-policies/scratch/ui/agent-credential.png');
		await page.getByRole('tab', { name: en.resources_tab_settings }).click();
		await expect(page.getByLabel(en.resources_agent_username)).toHaveValue('werbot_agent');
		await page.getByLabel(en.resources_agent_username).scrollIntoViewIfNeeded();
		await shot(page, 'sdd/stage18-sql-policies/scratch/ui/postgres-form.png');

		// the agent's database limits
		await page.goto('/agents');
		const row = page.getByRole('row').filter({ hasText: 'e2e-db-agent' });
		const limitsMenu = async () => {
			await row.getByRole('button', { name: en.common_more }).click();
			await page.getByRole('menuitem', { name: en.agents_action_limits, exact: true }).click();
		};
		await limitsMenu();
		const limits = page.getByRole('dialog', { name: new RegExp(en.agents_action_limits) });
		await limits.getByLabel(en.agents_limits_max_rows).fill('500');
		await limits.getByLabel(en.agents_limits_statement_timeout).fill('45s');
		await expect(page.getByRole('menu')).toHaveCount(0);
		await shot(page, 'sdd/stage18-sql-policies/scratch/ui/limits.png');
		await limits.getByRole('button', { name: en.settings_save }).click();
		await expect(limits).toHaveCount(0);
		await limitsMenu();
		await expect(limits.getByLabel(en.agents_limits_max_rows)).toHaveValue('500');
		await expect(limits.getByLabel(en.agents_limits_statement_timeout)).toHaveValue('45s');
		await limits.getByRole('button', { name: en.common_cancel }).click();

		// a deny rule for destructive queries, then the SQL test box
		await page.goto('/policies/commands');
		await expect(page.getByRole('heading', { name: en.policies_commands_title })).toBeVisible();
		await page.getByRole('button', { name: en.policies_commands_new }).click();
		const drawer = page.getByRole('dialog', { name: en.policies_commands_new });
		await drawer.getByLabel(en.policies_commands_name).fill('no-drop');
		await drawer.getByText(en.policies_commands_all_users, { exact: true }).click();
		await choose(drawer.getByLabel(en.approvals_add_resource), 'e2e-db');
		await drawer.getByText(en.class_destructive, { exact: true }).click();
		await drawer.getByRole('button', { name: en.policies_commands_create }).click();
		await expect(drawer).toHaveCount(0);

		const probe = page.locator('section').filter({ has: page.getByRole('heading', { name: en.policies_test_title }) });
		await choose(probe.getByLabel(en.policies_test_user), 'Owner (owner)');
		await choose(probe.getByLabel(en.policies_test_resource), 'e2e-db');
		await expect(probe.getByLabel(en.policies_test_cwd)).toHaveCount(0);
		await probe.getByLabel(en.policies_test_sql).fill('DROP TABLE t');
		const result = probe.getByTestId('test-result');
		// the server warms its SQL parser on first use; until then the text is opaque
		await expect(async () => {
			await probe.getByRole('button', { name: en.policies_test_run }).click();
			await expect(result.getByText(en.class_destructive, { exact: true })).toBeVisible({ timeout: 1000 });
		}).toPass({ timeout: 30_000 });
		await expect(result.getByText(en.policies_test_denied)).toBeVisible();
		await expect(result.getByText('no-drop', { exact: true })).toBeVisible();
		await result.scrollIntoViewIfNeeded();
		await shot(page, 'sdd/stage18-sql-policies/scratch/ui/sql-test.png');
	} finally {
		for (const r of (await rpc(page, svc + 'ListCommandRules', {})).rules ?? []) if (r.name === 'no-drop') await rpc(page, svc + 'DeleteCommandRule', { id: r.id });
		await rpc(page, agentSvc + 'DeleteAgent', { id: agent.id });
		await rpc(page, 'werbot.v1.ResourceService/DeleteResource', { id: resource.id });
	}
});
