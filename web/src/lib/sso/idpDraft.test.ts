import { expect, test } from 'vitest';
import { fromDraft, mappingInvalid, toDraft } from './idpDraft.js';

test('a new provider is enabled with the default scopes and no mapping', () => {
	const d = toDraft();
	expect(d).toMatchObject({ enabled: true, allowSignup: false, scopes: 'openid email profile', mapping: [] });
});

test('draft keeps an existing provider and maps claim rows to teams and roles', () => {
	const d = toDraft({
		name: 'Keycloak',
		issuer: 'https://idp',
		clientId: 'werbot',
		scopes: ['openid', 'email'],
		enabled: true,
		allowSignup: true,
		claimsMapping: [
			{ claim: 'groups', regex: '^/ops$', target: { case: 'teamId', value: 't1' } },
			{ claim: 'groups', regex: '^/admins$', target: { case: 'role', value: 'admin' } }
		]
	} as never);
	expect(d.mapping.map((r) => [r.kind, r.target])).toEqual([['team', 't1'], ['role', 'admin']]);
	const out = fromDraft(d);
	expect(out.scopes).toEqual(['openid', 'email']);
	expect(out.claimsMapping).toEqual([
		{ claim: 'groups', regex: '^/ops$', target: { case: 'teamId', value: 't1' } },
		{ claim: 'groups', regex: '^/admins$', target: { case: 'role', value: 'admin' } }
	]);
});

test('scopes split on spaces and commas; an empty list is sent empty (server default)', () => {
	const d = toDraft();
	d.scopes = 'openid,  email  groups';
	expect(fromDraft(d).scopes).toEqual(['openid', 'email', 'groups']);
	d.scopes = '  ';
	expect(fromDraft(d).scopes).toEqual([]);
});

test('an empty client secret is sent empty (update keeps the stored one)', () => {
	expect(fromDraft(toDraft()).clientSecret).toBe('');
});

test('a mapping row needs claim, a valid regex and a target', () => {
	const ok = { claim: 'groups', regex: '^/ops$', kind: 'team', target: 't1' };
	expect(mappingInvalid(ok)).toBe(false);
	expect(mappingInvalid({ ...ok, claim: '' })).toBe(true);
	expect(mappingInvalid({ ...ok, regex: '(' })).toBe(true);
	expect(mappingInvalid({ ...ok, target: '' })).toBe(true);
});
