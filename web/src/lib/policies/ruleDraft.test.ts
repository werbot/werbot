import { expect, test } from 'vitest';
import { allowlistDraft, fromDraft, presetDraft, toDraft } from './ruleDraft';

test('new draft denies by resource', () => {
	const d = toDraft();
	expect(d.effect).toBe('deny');
	expect(d.by).toBe('resources');
});

test('labels selector wins when the rule has labels', () => {
	const d = toDraft({ name: 'r', classes: ['write'], effect: 'allow', resourceSelector: { labels: { env: 'prod' }, resourceIds: [] } } as never);
	expect(d.by).toBe('labels');
	expect(fromDraft(d).resourceSelector).toEqual({ resourceIds: [], labels: { env: 'prod' } });
});

test('preset asks approval for opaque commands from all users', () => {
	const r = fromDraft(presetDraft());
	expect(r).toMatchObject({ classes: ['opaque'], effect: 'approval', subjects: { allUsers: true } });
});

test('allowlist preset denies opaque commands from all users', () => {
	const r = fromDraft(allowlistDraft('Only listed commands'));
	expect(r).toMatchObject({ name: 'only-allowed-commands', classes: ['opaque'], effect: 'deny', message: 'Only listed commands', subjects: { allUsers: true } });
});
