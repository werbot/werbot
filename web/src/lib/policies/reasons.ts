import { m } from '#lib/paraglide/messages.js';

const named = new Set(['command', 'custom', 'path', 'terminal']);

const baselines: Record<string, () => string> = {
	connect: m.agents_baseline_connect,
	read: m.agents_baseline_read,
	dangerous: m.agents_baseline_dangerous,
	terminal: m.agents_baseline_terminal,
	sftp: m.agents_baseline_sftp,
	frozen: m.agents_baseline_frozen,
	blocked_runs: m.agents_baseline_blocked_runs
};

/** `command:<id>[:agent]`, `path:<id>` and `terminal:<id>[:deny]` name a rule, `custom:<id>:<n>` a custom policy, `agent:baseline:<x>` an agent baseline; anything else is shown as the server sent it. */
export function reasonName(reason: string, names: ReadonlyMap<string, string>): string {
	const [kind, id, rest] = reason.split(':');
	if (kind === 'agent' && id === 'baseline') return baselines[rest]?.() ?? reason;
	return named.has(kind) && names.get(id) ? names.get(id)! : reason;
}
