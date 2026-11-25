import { execFileSync, spawn } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { expect, test } from '@playwright/test';
import en from '../messages/en.json' with { type: 'json' };
import { BOB, loginAsOwner, loginWithNext } from './support/auth';
import { rpc } from './support/rpc';

const svc = 'werbot.v1.PolicyService/';

let dir = ''; // holds the generated private key
test.afterAll(() => {
	if (dir) rmSync(dir, { recursive: true, force: true });
});

test('command waits for approval, approved in the UI, then runs', async ({ page, browser, baseURL }) => {
	test.setTimeout(120_000);
	await loginAsOwner(page);
	// the embedded gateway registers its "local" pool shortly after the server starts
	const pool = async () =>
		((await rpc(page, 'werbot.v1.GatewayService/ListGateways', {})).gateways ?? []).find((g: { name: string }) => g.name === 'local');
	await expect.poll(pool, { timeout: 30_000 }).toBeTruthy();
	const local = await pool();
	dir = mkdtempSync(join(tmpdir(), 'werbot-e2e-'));
	execFileSync('ssh-keygen', ['-q', '-t', 'ed25519', '-N', '', '-f', join(dir, 'id')]);
	const { key } = await rpc(page, 'werbot.v1.SSHKeyService/AddSSHKey', { name: 'e2e-exec', publicKey: readFileSync(join(dir, 'id.pub'), 'utf8') });
	const { resource } = await rpc(page, 'werbot.v1.ResourceService/CreateResource', {
		name: 'e2e-approve',
		type: 'ssh',
		gatewayId: local.id,
		config: { '@type': 'type.googleapis.com/werbot.connector.ssh.v1.Config', host: '127.0.0.1', port: 2022, login: 'e2e' }
	});

	try {
		await rpc(page, 'werbot.v1.ResourceService/SetCredential', { resourceId: resource.id, kind: 'password', value: btoa('e2e-secret') });
		const { user } = await rpc(page, 'werbot.v1.AuthService/GetSession', {});
		const { members } = await rpc(page, 'werbot.v1.MemberService/ListMembers', {});
		const bobID = members.find((m: { user: { email: string } }) => m.user.email === BOB.email).user.id;
		await rpc(page, svc + 'CreateCommandRule', {
			name: 'e2e-read-approval',
			effect: 'approval',
			subjects: { allUsers: true },
			resourceSelector: { resourceIds: [resource.id] },
			classes: ['read']
		});
		await rpc(page, 'werbot.v1.ApprovalPolicyService/CreateApprovalPolicy', {
			name: 'e2e-commands',
			resourceSelector: { resourceIds: [resource.id] },
			requesters: { allUsers: true },
			approvers: { users: [bobID] },
			mode: 'manual'
		});

		// the binding comes last: a snapshot that lets the owner in already holds the rule
		await rpc(page, 'werbot.v1.AccessService/CreateBinding', { subjectType: 'user', subjectId: user.id, resourceId: resource.id });

		// the owner's command waits; the gateway prints the approval id and the approvers' page
		const run = () => {
			const ssh = spawn('ssh', [
				'-F', '/dev/null', '-i', join(dir, 'id'), '-p', '2222', '-o', 'BatchMode=yes', '-o', 'IdentitiesOnly=yes',
				'-o', 'ConnectTimeout=3', '-o', 'StrictHostKeyChecking=no', '-o', 'UserKnownHostsFile=/dev/null',
				`${user.login}+e2e-approve@127.0.0.1`, 'echo werbot-approved'
			]);
			const out = { ssh, stdout: '', stderr: '', code: null as number | null, done: false };
			ssh.stdout.on('data', (b) => (out.stdout += b));
			ssh.stderr.on('data', (b) => (out.stderr += b));
			ssh.on('close', (c) => Object.assign(out, { code: c, done: true }));
			return out;
		};
		const waiting = /Waiting for approval \S+, timeout 5m/;
		let cmd = run();
		// until the gateway sees the new resource (snapshot debounce) the owner is refused: try again
		await expect
			.poll(
				() => {
					if (cmd.done && !waiting.test(cmd.stderr)) cmd = run();
					return cmd.stderr;
				},
				{ timeout: 20_000 }
			)
			.toMatch(waiting);
		try {
			const stderr = cmd.stderr;
			const url = stderr.match(/http:\/\/localhost:18080\/requests\?id=[0-9a-f-]{36}/)?.[0];
			expect(url).toBeTruthy();

			// the requester sees the command and its class, and cannot cancel a command approval
			await page.goto(url!);
			const own = page.getByRole('dialog', { name: 'e2e-approve' });
			await expect(own.getByText('echo werbot-approved')).toBeVisible();
			await expect(own.getByText(en.class_read, { exact: true })).toBeVisible();
			await expect(own.getByRole('button', { name: en.common_cancel })).toHaveCount(0);
			await expect(page.getByRole('row').filter({ hasText: 'e2e-approve' })).not.toContainText(en.requests_duration_default);

			// bob, the policy's approver, approves it from the link in his own browser context
			const bobContext = await browser.newContext({ baseURL });
			const bob = await bobContext.newPage();
			await bob.goto(url!);
			await loginWithNext(bob, BOB);
			const drawer = bob.getByRole('dialog', { name: 'e2e-approve' });
			await expect(drawer.getByText('echo werbot-approved')).toBeVisible();
			await drawer.getByRole('button', { name: en.requests_approve }).click();
			await expect(bob.getByText(en.requests_approved)).toBeVisible();
			await bobContext.close();

			await expect.poll(() => cmd.done, { timeout: 20_000 }).toBe(true);
			expect(cmd.code).toBe(0);
			expect(cmd.stdout).toContain('werbot-approved');
		} finally {
			cmd.ssh.kill();
		}

		// the session list shows the command and its class
		await page.goto('/sessions');
		const row = page.getByRole('row').filter({ hasText: 'echo werbot-approved' });
		await expect(row).toBeVisible();
		await expect(row.getByText(en.class_read, { exact: true })).toBeVisible();
	} finally {
		// later specs share the database: no rule, no policy, no resource, no key
		for (const r of (await rpc(page, svc + 'ListCommandRules', {})).rules ?? []) await rpc(page, svc + 'DeleteCommandRule', { id: r.id });
		const { approvalPolicies } = await rpc(page, 'werbot.v1.ApprovalPolicyService/ListApprovalPolicies', {});
		for (const p of approvalPolicies ?? []) await rpc(page, 'werbot.v1.ApprovalPolicyService/DeleteApprovalPolicy', { id: p.id });
		await rpc(page, 'werbot.v1.ResourceService/DeleteResource', { id: resource.id });
		await rpc(page, 'werbot.v1.SSHKeyService/DeleteSSHKey', { id: key.id });
	}
});
