import { timestampDate } from '@bufbuild/protobuf/wkt';
import type { Timestamp } from '@bufbuild/protobuf/wkt';

export function fmt(ts?: Timestamp): string {
	return ts ? timestampDate(ts).toLocaleString() : '—';
}
