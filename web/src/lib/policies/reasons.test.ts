import { describe, expect, it } from 'vitest';
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
	it('keeps unknown reasons as is', () => {
		expect(reasonName('command:gone', names)).toBe('command:gone');
		expect(reasonName('default', names)).toBe('default');
	});
});
