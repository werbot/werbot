import { timestampMs, type Timestamp } from '@bufbuild/protobuf/wkt';

/** Whole seconds until `expiresAt`, never negative. */
export function secondsLeft(expiresAt: Timestamp | undefined, now = Date.now()): number {
	return expiresAt ? Math.max(0, Math.ceil((timestampMs(expiresAt) - now) / 1000)) : 0;
}

/** 75 → "1:15" */
export function countdown(seconds: number): string {
	return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, '0')}`;
}
