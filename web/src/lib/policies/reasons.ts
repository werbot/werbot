const named = new Set(['command', 'custom', 'path', 'terminal']);

/** `command:<id>`, `path:<id>` and `terminal:<id>[:deny]` name a rule, `custom:<id>:<n>` a custom policy; anything else is shown as the server sent it. */
export function reasonName(reason: string, names: ReadonlyMap<string, string>): string {
	const [kind, id] = reason.split(':');
	return named.has(kind) && names.get(id) ? names.get(id)! : reason;
}
