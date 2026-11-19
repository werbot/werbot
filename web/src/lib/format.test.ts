import { timestampFromMs } from '@bufbuild/protobuf/wkt';
import { expect, test } from 'vitest';
import { ago, elapsed } from './format';

const now = Date.UTC(2026, 10, 19, 12, 0, 0);
const at = (ms: number) => timestampFromMs(now + ms);
const sec = 1000;
const min = 60 * sec;
const hour = 60 * min;
const day = 24 * hour;

test('ago picks the largest whole unit before now', () => {
	expect(ago(at(0), now, 'en')).toBe('now');
	expect(ago(at(-42 * sec), now, 'en')).toBe('42 seconds ago');
	expect(ago(at(-59.6 * sec), now, 'en')).toBe('59 seconds ago');
	expect(ago(at(-5 * min), now, 'en')).toBe('5 minutes ago');
	expect(ago(at(-3 * hour), now, 'en')).toBe('3 hours ago');
	expect(ago(at(-day), now, 'en')).toBe('1 day ago');
	expect(ago(at(-36 * hour), now, 'en')).toBe('1 day ago');
	expect(ago(at(-40 * day), now, 'en')).toBe('1 month ago');
	expect(ago(at(-364 * day), now, 'en')).toBe('1 year ago');
	expect(ago(at(-800 * day), now, 'en')).toBe('2 years ago');
});

test('ago of a future time is now', () => {
	expect(ago(at(2 * sec), now, 'en')).toBe('now');
});

test('ago of a missing time is a dash', () => {
	expect(ago(undefined, now, 'en')).toBe('—');
});

test('elapsed shows the two largest units of a span', () => {
	expect(elapsed(at(0), at(42 * sec), 'en')).toBe('42s');
	expect(elapsed(at(0), at(3 * min + 12 * sec), 'en')).toBe('3m 12s');
	expect(elapsed(at(0), at(hour + 5 * min + 30 * sec), 'en')).toBe('1h 5m');
	expect(elapsed(at(0), at(2 * hour), 'en')).toBe('2h');
	expect(elapsed(at(0), at(2 * day + 3 * hour), 'en')).toBe('2d 3h');
	expect(elapsed(at(0), at(0), 'en')).toBe('0s');
});

test('elapsed without both ends is a dash', () => {
	expect(elapsed(at(0), undefined, 'en')).toBe('—');
	expect(elapsed(undefined, at(0), 'en')).toBe('—');
});
