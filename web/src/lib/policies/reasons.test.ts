import { describe, expect, it } from 'vitest';
import { reasonName } from './reasons.js';

const names = new Map([
	['r1', 'Block rm'],
	['p1', 'Night policy']
]);

describe('reasonName', () => {
	it('maps rule and custom policy ids to names', () => {
		expect(reasonName('command:r1', names)).toBe('Block rm');
		expect(reasonName('custom:p1:2', names)).toBe('Night policy');
	});
	it('keeps unknown reasons as is', () => {
		expect(reasonName('command:gone', names)).toBe('command:gone');
		expect(reasonName('default', names)).toBe('default');
	});
});
