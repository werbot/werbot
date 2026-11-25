import { describe, expect, it } from 'vitest';
import { m } from '#lib/paraglide/messages.js';
import { reasonName } from './reasons.js';

const names = new Map([
	['r1', 'Block rm'],
	['p1', 'Night policy'],
	['f1', 'Work folder'],
	['t1', 'Console only']
]);

describe('reasonName', () => {
	it('maps rule and custom policy ids to names', () => {
		expect(reasonName('command:r1', names)).toBe('Block rm');
		expect(reasonName('custom:p1:2', names)).toBe('Night policy');
	});
	it('maps path and terminal rule ids to names', () => {
		expect(reasonName('path:f1', names)).toBe('Work folder');
		expect(reasonName('terminal:t1', names)).toBe('Console only');
		expect(reasonName('terminal:t1:deny', names)).toBe('Console only');
	});
	it('maps agent rule and baseline reasons', () => {
		expect(reasonName('command:r1:agent', names)).toBe('Block rm');
		expect(reasonName('agent:baseline:terminal', names)).toBe(m.agents_baseline_terminal());
		expect(reasonName('agent:baseline:blocked_runs', names)).toBe(m.agents_baseline_blocked_runs());
	});
	it('keeps unknown reasons as is', () => {
		expect(reasonName('agent:baseline:gone', names)).toBe('agent:baseline:gone');
		expect(reasonName('command:gone', names)).toBe('command:gone');
		expect(reasonName('default', names)).toBe('default');
	});
});
