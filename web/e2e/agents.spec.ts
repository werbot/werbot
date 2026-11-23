import { create, toBinary } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { expect, test } from '@playwright/test';
import { GetRunResponseSchema } from '../src/lib/gen/werbot/v1/agent_pb';
import en from '../messages/en.json' with { type: 'json' };
import { loginAsOwner } from './support/auth';
import { rpc } from './support/rpc';

const agentSvc = 'werbot.v1.AgentService/';
const profileSvc = 'werbot.v1.AgentProfileService/';
const lockSvc = 'werbot.v1.LockService/';

test('owner manages agent limits, freeze and kill switch', async ({ page }) => {
	await loginAsOwner(page);
	const { agent } = await rpc(page, agentSvc + 'CreateAgent', { name: 'e2e-agent', limits: { maxActions: 5 } });
	let deleted = false;
	try {
		await page.goto('/agents');
		const row = page.getByRole('row').filter({ hasText: 'e2e-agent' });
		await expect(row.getByText(en.agents_status_active)).toBeVisible();
		const menu = async (item: string) => {
			await row.getByRole('button', { name: en.common_more }).click();
			await page.getByRole('menuitem', { name: item, exact: true }).click();
		};

		await menu(en.agents_action_limits);
		const limits = page.getByRole('dialog', { name: new RegExp(en.agents_action_limits) });
		await limits.getByLabel(en.agents_limits_max_actions).fill('7');
		await limits.getByRole('button', { name: en.settings_save }).click();
		await expect(limits).toHaveCount(0);
		await page.reload();
		await expect(row).toContainText('7 /');

		await menu(en.agents_action_freeze);
		await expect(row.getByText(en.agents_status_frozen)).toBeVisible();

		await menu(en.agents_kill_agent);
		const kill = page.getByRole('dialog', { name: new RegExp(en.agents_kill_agent) });
		await expect(kill.getByRole('button', { name: en.agents_kill_confirm })).toBeDisabled();
		await kill.getByLabel(en.agents_kill_reason).fill('e2e kill switch');
		await kill.getByRole('button', { name: en.agents_kill_confirm }).click();
		await expect(page.getByText(en.locks_created)).toBeVisible();

		await page.goto('/locks');
		await expect(page.getByRole('row').filter({ hasText: 'e2e-agent' })).toContainText(en.locks_type_agent);

		await page.goto('/agents');
		await menu(en.agents_action_delete);
		await page.getByRole('dialog', { name: en.agents_action_delete }).getByRole('button', { name: en.agents_action_delete }).click();
		deleted = true;
		await expect(row).toHaveCount(0);
	} finally {
		const { locks = [] } = await rpc(page, lockSvc + 'ListLocks', {});
		for (const l of locks.filter((x: { targetId: string }) => x.targetId === agent.id)) await rpc(page, lockSvc + 'DeleteLock', { id: l.id });
		if (!deleted) await rpc(page, agentSvc + 'DeleteAgent', { id: agent.id });
	}
});

test('owner sees and revokes agent profiles', async ({ page }) => {
	await loginAsOwner(page);
	const { agent } = await rpc(page, agentSvc + 'CreateAgent', { name: 'e2e-profile' });
	try {
		await rpc(page, profileSvc + 'CreateAgentProfile', { agentId: agent.id, label: 'e2e box' });
		await page.goto('/agents');
		const row = page.getByRole('row').filter({ hasText: 'e2e-profile' });
		await row.getByRole('button', { name: en.common_more }).click();
		await page.getByRole('menuitem', { name: en.agents_action_profiles, exact: true }).click();
		const drawer = page.getByRole('dialog', { name: 'Profiles of e2e-profile' });
		await expect(drawer.getByText('werbot agents profile create e2e-profile')).toBeVisible();
		await expect(drawer.getByRole('row').filter({ hasText: 'e2e box' })).toBeVisible();
		await page.waitForTimeout(500); // the drawer slide-in ends
		await page.screenshot({ path: '../.superpowers/sdd/stage12-agents/screens/agent-profiles-drawer.png' });

		await drawer.getByRole('button', { name: en.agents_profiles_revoke }).click();
		await page.getByRole('dialog', { name: en.agents_profiles_revoke }).getByRole('button', { name: en.agents_profiles_revoke }).click();
		await expect(page.getByText(en.agents_profiles_revoked)).toBeVisible();
		await expect(drawer.getByText(en.agents_profiles_empty)).toBeVisible();
	} finally {
		await rpc(page, agentSvc + 'DeleteAgent', { id: agent.id });
	}
});

test('unknown run shows not found', async ({ page }) => {
	await loginAsOwner(page);
	await page.goto('/agents/runs/019d0000-0000-7000-8000-000000000000');
	await expect(page.getByText(en.agents_run_not_found)).toBeVisible();
});

test('run timeline filters events', async ({ page }) => {
	await loginAsOwner(page);
	const at = (s: number) => timestampFromDate(new Date(Date.UTC(2026, 10, 23, 10, 0, s)));
	const body = toBinary(
		GetRunResponseSchema,
		create(GetRunResponseSchema, {
			run: { id: '019d0000-0000-7000-8000-000000000001', agentName: 'e2e-agent', status: 'ended', actions: 2, maxActions: 5, client: 'e2e-client', startedAt: at(0), expiresAt: at(3600) },
			toolCalls: [{ id: 't1', tool: 'exec_command', status: 'ok', startedAt: at(1) }],
			denials: [{ command: 'cat /etc/shadow', classes: ['secrets'], reasons: ['agent:baseline:dangerous'], at: at(2) }]
		})
	);
	await page.route('**/werbot.v1.AgentService/GetRun', (r) => r.fulfill({ status: 200, contentType: 'application/proto', body: Buffer.from(body) }));
	await page.goto('/agents/runs/019d0000-0000-7000-8000-000000000001');
	await expect(page.getByText('exec_command')).toBeVisible();
	await expect(page.getByText(en.agents_timeline_client_reported)).toBeVisible();
	await expect(page.getByText(en.agents_baseline_dangerous)).toBeVisible();
	await page.getByLabel(en.common_filter).selectOption({ label: en.agents_timeline_filter_denials });
	await expect(page.getByText('exec_command')).toHaveCount(0);
	await expect(page.getByText('cat /etc/shadow')).toBeVisible();
});

test('custom cedar form shows the agents hint', async ({ page }) => {
	await loginAsOwner(page);
	await page.goto('/policies/custom');
	await page.getByRole('button', { name: en.policies_custom_new }).first().click();
	await expect(page.getByText(en.policies_custom_agents_hint)).toBeVisible();
});
