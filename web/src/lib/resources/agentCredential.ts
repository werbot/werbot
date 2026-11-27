import type { BadgeTone } from '#lib/ui/index.js';
import { m } from '#lib/paraglide/messages.js';

/** The credential slot agents use on a database resource. */
export const AGENT_SLOT = 'agent_readonly';

/** A type with a "<name>.query" action takes SQL: an agent credential slot, SQL rule tests. */
export const hasQuery = (types: readonly { name: string; actions: readonly string[] }[], type: string): boolean =>
	types.find((t) => t.name === type)?.actions.includes(`${type}.query`) ?? false;

const statuses: Record<string, { tone: BadgeTone; label: () => string }> = {
	none: { tone: 'ghost', label: m.resources_agent_status_none },
	pending: { tone: 'upcoming', label: m.resources_agent_status_pending },
	ok: { tone: 'accent', label: m.resources_agent_status_ok },
	has_write: { tone: 'alert', label: m.resources_agent_status_has_write },
	error: { tone: 'alert', label: m.resources_agent_status_error }
};

/** The badge of an agent_readonly status; empty is none, an unknown status (a newer server) is shown as sent. */
export function agentStatus(status: string): { tone: BadgeTone; text: string } {
	const s = statuses[status || 'none'];
	return s ? { tone: s.tone, text: s.label() } : { tone: 'base', text: status };
}

/** Comma separated globs, trimmed, empty items dropped. */
export const splitList = (text: string): string[] =>
	text
		.split(',')
		.map((s) => s.trim())
		.filter(Boolean);
