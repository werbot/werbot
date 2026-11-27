import { expect, test } from 'vitest';
import { agentStatus, hasQuery, splitList } from './agentCredential';

const types = [
	{ name: 'postgres', actions: ['postgres.connect', 'postgres.query'] },
	{ name: 'ssh', actions: ['ssh.connect', 'ssh.exec'] }
];

test('only a type with its query action takes SQL', () => {
	expect(hasQuery(types, 'postgres')).toBe(true);
	expect(hasQuery(types, 'ssh')).toBe(false);
	expect(hasQuery(types, 'unknown')).toBe(false);
	expect(hasQuery([{ name: 'x', actions: ['postgres.query'] }], 'x')).toBe(false);
});

test('agent statuses map to badge tones', () => {
	expect(agentStatus('ok').tone).toBe('accent');
	expect(agentStatus('pending').tone).toBe('upcoming');
	expect(agentStatus('has_write').tone).toBe('alert');
	expect(agentStatus('error').tone).toBe('alert');
	expect(agentStatus('none').tone).toBe('ghost');
	expect(agentStatus('')).toEqual(agentStatus('none'));
	expect(agentStatus('future')).toEqual({ tone: 'base', text: 'future' });
});

test('lists split on commas and drop empty items', () => {
	expect(splitList(' public.secrets ,, *.tokens ,')).toEqual(['public.secrets', '*.tokens']);
	expect(splitList('')).toEqual([]);
});
