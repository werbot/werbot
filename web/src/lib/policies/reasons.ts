/** `command:<id>` names a command rule, `custom:<id>:<n>` a custom policy; anything else is shown as the server sent it. */
export function reasonName(reason: string, names: ReadonlyMap<string, string>): string {
	const [kind, id] = reason.split(':');
	return (kind === 'command' || kind === 'custom') && names.get(id) ? names.get(id)! : reason;
}
