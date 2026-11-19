import { timestampFromDate, type Timestamp } from '@bufbuild/protobuf/wkt';

// dayStart turns an <input type="date"> value into local midnight, plus addDays
// days (1 makes an inclusive "to" day an exclusive bound); empty or invalid → undefined.
export function dayStart(value: string, addDays = 0): Timestamp | undefined {
	const [y, m, d] = value.split('-').map(Number);
	const date = new Date(y, m - 1, d + addDays);
	return value && !Number.isNaN(date.getTime()) ? timestampFromDate(date) : undefined;
}

// dayString is the inverse for a Calendar date: local YYYY-MM-DD, empty for null.
export function dayString(date: Date | null): string {
	return date ? `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}` : '';
}
