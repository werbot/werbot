import { expect, test } from 'vitest';
import { durationFromMs } from '@bufbuild/protobuf/wkt';
import { formatDuration, parseDuration } from './duration';

test('parse and format durations', () => {
	expect(formatDuration(parseDuration('1h30m')!)).toBe('1h30m');
	expect(parseDuration('14d')!.seconds).toBe(1209600n);
	expect(parseDuration('2x')).toBeNull();
});

test('parse accepts units largest first, each once', () => {
	expect(parseDuration(' 30s ')!.seconds).toBe(30n);
	expect(parseDuration('1d2h3m4s')!.seconds).toBe(93784n);
	for (const bad of ['', '2', 'h', '1m1h', '1h1h', '-1h', '1.5h']) expect(parseDuration(bad), bad).toBeNull();
});

test('format uses the largest units', () => {
	expect(formatDuration(durationFromMs(7_200_000))).toBe('2h');
	expect(formatDuration(durationFromMs(5_400_000))).toBe('1h30m');
	expect(formatDuration(durationFromMs(30 * 86_400_000))).toBe('30d');
	expect(formatDuration(durationFromMs(0))).toBe('0s');
	expect(formatDuration(undefined)).toBe('');
});
