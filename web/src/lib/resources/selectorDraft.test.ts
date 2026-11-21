import { expect, test } from 'vitest';
import { fromSelectorDraft, fromSubjectsDraft, toSelectorDraft, toSubjectsDraft } from './selectorDraft';

test('labels selector wins when there are labels; only the chosen selector is sent', () => {
	const d = toSelectorDraft({ labels: { env: 'prod' }, resourceIds: ['a'] } as never);
	expect(d.by).toBe('labels');
	expect(fromSelectorDraft(d)).toEqual({ resourceIds: [], labels: { env: 'prod' } });
	expect(fromSelectorDraft({ ...d, by: 'resources' })).toEqual({ resourceIds: ['a'], labels: {} });
	expect(toSelectorDraft().by).toBe('resources');
});

test('subjects copy their lists', () => {
	const d = toSubjectsDraft({ users: ['u'], teams: [], allUsers: true, allAgents: false } as never);
	expect(d).toEqual({ users: ['u'], teams: [], allUsers: true, allAgents: false });
	expect(fromSubjectsDraft(d).users).not.toBe(d.users);
	expect(toSubjectsDraft()).toEqual({ users: [], teams: [], allUsers: false, allAgents: false });
});
