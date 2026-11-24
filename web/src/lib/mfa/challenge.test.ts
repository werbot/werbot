import { expect, test } from 'vitest';
import { secondsLeft, countdown } from './challenge.js';

test('secondsLeft counts down to zero and never goes negative', () => {
	const exp = { seconds: 1000n, nanos: 0 } as never;
	expect(secondsLeft(exp, 940_000)).toBe(60);
	expect(secondsLeft(exp, 1_000_000)).toBe(0);
	expect(secondsLeft(exp, 2_000_000)).toBe(0);
	expect(secondsLeft(undefined, 0)).toBe(0);
});

test('countdown formats m:ss', () => {
	expect(countdown(75)).toBe('1:15');
	expect(countdown(5)).toBe('0:05');
	expect(countdown(0)).toBe('0:00');
});
