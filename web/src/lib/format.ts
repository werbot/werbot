import { timestampDate, timestampMs } from '@bufbuild/protobuf/wkt';
import type { Timestamp } from '@bufbuild/protobuf/wkt';
import { getLocale } from '#lib/paraglide/runtime.js';

export function fmt(ts?: Timestamp): string {
	return ts ? timestampDate(ts).toLocaleString() : '—';
}

export function fmtShort(ts?: Timestamp): string {
	return ts ? timestampDate(ts).toLocaleString(undefined, { dateStyle: 'short', timeStyle: 'short' }) : '—';
}

const steps: [Intl.RelativeTimeFormatUnit, number][] = [
	['second', 60],
	['minute', 60],
	['hour', 24],
	['day', 30],
	['month', 12],
	['year', Infinity]
];

/** "5 minutes ago": the largest whole unit between ts and now; a future ts (clock skew) is "now". */
export function ago(ts?: Timestamp, now = Date.now(), locale = getLocale()): string {
	if (!ts) return '—';
	let v = Math.min(0, timestampMs(ts) - now) / 1000;
	let i = 0;
	while (Math.abs(v) >= steps[i][1]) v /= steps[i++][1];
	const n = Math.trunc(v);
	// 'auto' only for "now": in larger units it turns 36 hours into "yesterday"
	return new Intl.RelativeTimeFormat(locale, { numeric: n === 0 ? 'auto' : 'always' }).format(n, steps[i][0]);
}

export const spans: [string, number][] = [
	['day', 86400],
	['hour', 3600],
	['minute', 60],
	['second', 1]
];

/** "1h 5m": the two largest units of the span between from and to. */
export function elapsed(from?: Timestamp, to?: Timestamp, locale = getLocale()): string {
	if (!from || !to) return '—';
	let s = Math.max(0, Math.floor((timestampMs(to) - timestampMs(from)) / 1000));
	const counts = spans.map(([, size]) => {
		const n = Math.floor(s / size);
		s -= n * size;
		return n;
	});
	const found = counts.findIndex((n) => n > 0);
	const top = found < 0 ? spans.length - 1 : found;
	const unit = (i: number) => new Intl.NumberFormat(locale, { style: 'unit', unit: spans[i][0], unitDisplay: 'narrow' }).format(counts[i]);
	return counts[top + 1] ? `${unit(top)} ${unit(top + 1)}` : unit(top);
}

export function bytes(n: bigint | number): string {
	let v = Number(n);
	const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
	let i = 0;
	while (v >= 1024 && i < units.length - 1) {
		v /= 1024;
		i++;
	}
	return `${i ? v.toFixed(1) : v} ${units[i]}`;
}
