import { expect, test } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { durationFromMs } from '@bufbuild/protobuf/wkt';
import { ApprovalPolicySchema } from '#lib/gen/werbot/v1/approval_policy_pb.js';
import { fromDraft, toDraft } from './policyDraft';

test('a new draft asks for a reason and selects by resource', () => {
	const d = toDraft();
	expect(d).toMatchObject({ name: '', by: 'resources', mode: 'manual', maxDuration: '', requireReason: true });
	expect(fromDraft(d)!.maxDuration).toBeUndefined();
});

test('round trip keeps every field', () => {
	const policy = create(ApprovalPolicySchema, {
		id: 'p1',
		name: 'prod',
		resourceSelector: { labels: { env: 'prod' } },
		requesters: { users: ['u1'], teams: ['t1'], allUsers: true, allAgents: true },
		approvers: { users: ['u2'], teams: [] },
		mode: 'auto',
		maxDuration: durationFromMs(7_200_000),
		requireReason: false,
		fourEyes: true,
		breakGlass: true
	});
	const d = toDraft(policy);
	expect(d.by).toBe('labels');
	expect(d.maxDuration).toBe('2h');
	const out = fromDraft(d)!;
	expect(out.resourceSelector).toEqual({ resourceIds: [], labels: { env: 'prod' } });
	expect(out.requesters).toEqual({ users: ['u1'], teams: ['t1'], allUsers: true, allAgents: true });
	expect(out.approvers).toEqual({ users: ['u2'], teams: [], allUsers: false, allAgents: false });
	expect(out).toMatchObject({ name: 'prod', mode: 'auto', requireReason: false, fourEyes: true, breakGlass: true });
	expect(out.maxDuration?.seconds).toBe(7200n);
});

test('only the chosen selector is sent; a bad duration is refused', () => {
	const d = toDraft();
	d.resourceIds = ['r1'];
	d.labels = [{ key: 'env', value: 'prod' }];
	expect(fromDraft(d)!.resourceSelector).toEqual({ resourceIds: ['r1'], labels: {} });
	d.by = 'labels';
	expect(fromDraft(d)!.resourceSelector).toEqual({ resourceIds: [], labels: { env: 'prod' } });
	d.maxDuration = '2x';
	expect(fromDraft(d)).toBeNull();
});
