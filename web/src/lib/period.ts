import { timestampFromDate, type Timestamp } from '@bufbuild/protobuf/wkt';

// dayStart turns an <input type="date"> value into local midnight, plus addDays
// days (1 makes an inclusive "to" day an exclusive bound); empty or invalid → undefined.
export function dayStart(value: string, addDays = 0): Timestamp | undefined {
	const [y, m, d] = value.split('-').map(Number);
	const date = new Date(y, m - 1, d + addDays);
	return value && !Number.isNaN(date.getTime()) ? timestampFromDate(date) : undefined;
}
