import { timestampDate } from '@bufbuild/protobuf/wkt';
import { expect, test } from 'vitest';
import { dayStart, dayString } from './period';

test('dayStart is local midnight of the day, shifted by whole days', () => {
	expect(timestampDate(dayStart('2026-11-17')!)).toEqual(new Date(2026, 10, 17));
	expect(timestampDate(dayStart('2026-11-30', 1)!)).toEqual(new Date(2026, 11, 1));
});

test('dayStart of an empty or invalid value is undefined', () => {
	expect(dayStart('')).toBeUndefined();
	expect(dayStart('not-a-date')).toBeUndefined();
});

test('dayString is the local YYYY-MM-DD of a date, empty for null', () => {
	expect(dayString(new Date(2026, 10, 7))).toBe('2026-11-07');
	expect(dayString(null)).toBe('');
	expect(dayString(new Date(2026, 10, 7, 23, 59))).toBe('2026-11-07');
});
