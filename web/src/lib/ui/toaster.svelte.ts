/**
 * miso-ui toast state. Call `toast()` anywhere; render a single `<Toaster />`
 * near the app root to display the stack.
 *
 * Identical toasts (same tone + title + text + icon) are grouped: instead of
 * stacking a duplicate, the existing toast bumps its counter and restarts its
 * auto-dismiss timer. Pass `group: false` to always show a separate toast.
 */
import type { IconName } from './icons.js';

export type ToastTone =
	| 'accent'
	| 'violet'
	| 'neutral'
	| 'success'
	| 'warning'
	| 'alert'
	| 'invert';

export interface ToastAction {
	label: string;
	onclick: () => void;
}

export interface ToastOptions {
	title?: string;
	text?: string;
	tone?: ToastTone;
	/** overrides the tone default icon */
	icon?: IconName;
	/** auto-dismiss delay in ms; 0 keeps the toast until closed manually */
	duration?: number;
	/** collapse identical toasts into one with a counter (default true) */
	group?: boolean;
	action?: ToastAction;
}

export interface ToastItem {
	id: number;
	key: string;
	tone: ToastTone;
	title?: string;
	text?: string;
	icon: IconName;
	duration: number;
	/** how many identical toasts were grouped into this one */
	count: number;
	action?: ToastAction;
}

const defaultIcons: Record<ToastTone, IconName> = {
	accent: 'info',
	violet: 'bell',
	neutral: 'info',
	success: 'circle-check',
	warning: 'warning',
	alert: 'circle-alert',
	invert: 'lightning'
};

let nextId = 1;
const timers = new Map<number, ReturnType<typeof setTimeout>>();

export const toasts = $state<ToastItem[]>([]);

function armTimer(id: number, duration: number): void {
	const existing = timers.get(id);
	if (existing) clearTimeout(existing);
	if (duration > 0) {
		timers.set(
			id,
			setTimeout(() => dismissToast(id), duration)
		);
	}
}

export function toast(options: ToastOptions = {}): number {
	const tone = options.tone ?? 'neutral';
	const icon = options.icon ?? defaultIcons[tone];
	const duration = options.duration ?? 5000;
	const key = [tone, options.title ?? '', options.text ?? '', icon].join('\u0000');

	if (options.group !== false) {
		const same = toasts.find((item) => item.key === key);
		if (same) {
			same.count += 1;
			armTimer(same.id, duration);
			return same.id;
		}
	}

	const id = nextId++;
	toasts.push({
		id,
		key,
		tone,
		title: options.title,
		text: options.text,
		icon,
		duration,
		count: 1,
		action: options.action
	});
	armTimer(id, duration);
	return id;
}

export function dismissToast(id: number): void {
	const timer = timers.get(id);
	if (timer) {
		clearTimeout(timer);
		timers.delete(id);
	}
	const index = toasts.findIndex((item) => item.id === id);
	if (index !== -1) {
		toasts.splice(index, 1);
	}
}
