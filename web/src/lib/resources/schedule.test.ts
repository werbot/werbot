import { describe, expect, it } from 'vitest';
import { allKeys, dayKeys, slotKey, workKeys } from './schedule';

describe('schedule keys', () => {
	it('numbers a slot as weekday * 24 + hour', () => {
		expect(slotKey(0, 0)).toBe(0);
		expect(slotKey(6, 23)).toBe(167);
	});

	it('covers each hour of the week once', () => {
		const keys = allKeys();
		expect(new Set(keys).size).toBe(168);
		expect(dayKeys(1)).toEqual(Array.from({ length: 24 }, (_, h) => 24 + h));
	});

	it('makes working hours Monday to Friday, 09:00 to 18:00', () => {
		const keys = workKeys();
		expect(keys).toHaveLength(45);
		expect(keys).toContain(slotKey(1, 9));
		expect(keys).toContain(slotKey(5, 17));
		expect(keys).not.toContain(slotKey(1, 18));
		expect(keys).not.toContain(slotKey(0, 10));
		expect(keys).not.toContain(slotKey(6, 10));
	});
});
