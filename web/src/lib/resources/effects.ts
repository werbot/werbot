import type { ToneOption } from '#lib/ToneSegmented.svelte';
import { m } from '#lib/paraglide/messages.js';

export type Mode = 'allow' | 'deny';

/** the effect of a whole rule list: allow only its entries, or deny them */
export const modeOptions = (): ToneOption[] => [
	{ value: 'allow', label: m.rules_mode_allow(), tone: 'allow' },
	{ value: 'deny', label: m.rules_mode_deny(), tone: 'deny' }
];

/**
 * modeOf reads one effect for a rule list from its rules: the first rule's, 'allow' when empty.
 * mixed is true when the rules carry both effects (stored by an older form or the API).
 */
export function modeOf(effects: string[]): { mode: Mode; mixed: boolean } {
	return { mode: effects[0] === 'deny' ? 'deny' : 'allow', mixed: new Set(effects).size > 1 };
}
