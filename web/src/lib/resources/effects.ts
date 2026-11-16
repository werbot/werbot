import type { SegmentedOption } from '#lib/ui/index.js';
import { m } from '#lib/paraglide/messages.js';

export const effectOptions = (): SegmentedOption[] => [
	{ value: 'allow', label: m.rules_allow() },
	{ value: 'deny', label: m.rules_deny() }
];
