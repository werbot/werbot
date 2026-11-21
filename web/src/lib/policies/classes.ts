import type { LabelColor } from '#lib/ui/index.js';
import { m } from '#lib/paraglide/messages.js';

export const CLASSES = ['read', 'write', 'destructive', 'privilege', 'secrets', 'egress', 'opaque'] as const;
export type CommandClass = (typeof CLASSES)[number];
const colors: Record<CommandClass, LabelColor> = {
	read: 'coolgrey',
	write: 'blue',
	destructive: 'red',
	privilege: 'red',
	secrets: 'orange',
	egress: 'orange',
	opaque: 'orange'
};

const labels: Record<CommandClass, () => string> = {
	read: m.class_read,
	write: m.class_write,
	destructive: m.class_destructive,
	privilege: m.class_privilege,
	secrets: m.class_secrets,
	egress: m.class_egress,
	opaque: m.class_opaque
};

const known = (c: string): c is CommandClass => c in colors;

/** An unknown class (a newer server) is shown as the server sent it, neutral. */
export const classColor = (c: string): LabelColor => (known(c) ? colors[c] : 'coolgrey');
export const classLabel = (c: string): string => (known(c) ? labels[c]() : c);

export type Verdict = 'allowed' | 'denied' | 'approval';

/** A deny that carries the approval obligation waits for an approver. */
export const verdictOf = (r: { allow: boolean; obligations: string[] }): Verdict =>
	r.allow ? 'allowed' : r.obligations.includes('approval') ? 'approval' : 'denied';
