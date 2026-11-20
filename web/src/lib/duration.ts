import { durationFromMs, type Duration } from '@bufbuild/protobuf/wkt';

const units: [string, number][] = [
	['d', 86400],
	['h', 3600],
	['m', 60],
	['s', 1]
];
const pattern = /^(?:(\d+)d)?(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)s)?$/;

/** "2h", "30m", "1h30m", "14d" → Duration: units d, h, m, s, largest first, each at most once; anything else → null. */
export function parseDuration(s: string): Duration | null {
	const text = s.trim();
	const match = pattern.exec(text);
	if (!text || !match) return null;
	const seconds = units.reduce((sum, [, size], i) => sum + Number(match[i + 1] ?? 0) * size, 0);
	return durationFromMs(seconds * 1000);
}

/** Duration → "2h", "1h30m", "14d"; unset → "". Sub-second parts are dropped. */
export function formatDuration(d?: Duration): string {
	if (!d) return '';
	let rest = Number(d.seconds);
	const out = units.map(([unit, size]) => {
		const n = Math.floor(rest / size);
		rest -= n * size;
		return n ? `${n}${unit}` : '';
	});
	return out.join('') || '0s';
}
