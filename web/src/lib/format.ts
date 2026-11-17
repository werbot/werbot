import { timestampDate } from '@bufbuild/protobuf/wkt';
import type { Timestamp } from '@bufbuild/protobuf/wkt';

export function fmt(ts?: Timestamp): string {
	return ts ? timestampDate(ts).toLocaleString() : '—';
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
