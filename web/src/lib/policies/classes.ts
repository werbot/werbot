import { m } from '#lib/paraglide/messages.js';

export const CLASSES = ['read', 'write', 'destructive', 'privilege', 'secrets', 'egress', 'opaque'] as const;
export type CommandClass = (typeof CLASSES)[number];
export type ClassTone = 'neutral' | 'info' | 'warning' | 'alert';

const tones: Record<CommandClass, ClassTone> = {
	read: 'neutral',
	write: 'info',
	destructive: 'alert',
	privilege: 'alert',
	secrets: 'warning',
	egress: 'warning',
	opaque: 'warning'
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

const known = (c: string): c is CommandClass => c in tones;

/** An unknown class (a newer server) is shown as the server sent it, neutral. */
export const classTone = (c: string): ClassTone => (known(c) ? tones[c] : 'neutral');
export const classLabel = (c: string): string => (known(c) ? labels[c]() : c);

export type Verdict = 'allowed' | 'denied' | 'approval';

/** A deny that carries the approval obligation waits for an approver. */
export const verdictOf = (r: { allow: boolean; obligations: string[] }): Verdict =>
	r.allow ? 'allowed' : r.obligations.includes('approval') ? 'approval' : 'denied';
