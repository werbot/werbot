import { timestampDate } from '@bufbuild/protobuf/wkt';
import { expect, test } from 'vitest';
import { dayStart } from './period';

test('dayStart is local midnight of the day, shifted by whole days', () => {
	expect(timestampDate(dayStart('2026-11-17')!)).toEqual(new Date(2026, 10, 17));
	expect(timestampDate(dayStart('2026-11-30', 1)!)).toEqual(new Date(2026, 11, 1));
});

test('dayStart of an empty or invalid value is undefined', () => {
	expect(dayStart('')).toBeUndefined();
	expect(dayStart('not-a-date')).toBeUndefined();
});
