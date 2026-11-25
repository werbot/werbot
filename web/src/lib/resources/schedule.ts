// Schedule slot keys are weekday * 24 + hour; weekday 0 is Sunday, UTC.

/** display order of the weekdays: Monday first */
export const weekOrder = [1, 2, 3, 4, 5, 6, 0];

export const slotKey = (weekday: number, hour: number) => weekday * 24 + hour;

/** keys of every hour of one weekday */
export const dayKeys = (weekday: number) => Array.from({ length: 24 }, (_, h) => slotKey(weekday, h));

/** keys of the whole week */
export const allKeys = () => weekOrder.flatMap(dayKeys);

/** keys of Monday to Friday, 09:00 to 18:00 */
export const workKeys = () => [1, 2, 3, 4, 5].flatMap((d) => dayKeys(d).slice(9, 18));
