import type { BannerTone } from '#lib/ui/index.js';

/** One `.querylog` line (see docs/databases.md); the text is `sql`, or `command` for command, request and tool_call kinds. */
export type QueryLogEntry = {
	t: number;
	kind: string;
	sql?: string;
	command?: string;
	db?: string;
	params?: string[];
	statement?: string;
	classes?: string[];
	tables?: string[];
	decision?: string;
	policy_ids?: string[];
	rows?: number;
	bytes?: number;
	duration_us?: number;
	error?: string;
};

export type QueryLogFilter = { kind?: string; decision?: string; text?: string; errorsOnly?: boolean };

/** Rows shown before "show more". */
export const QUERYLOG_PAGE = 1000;

/** NDJSON to entries; blank lines are skipped, a line that is not a JSON object with a kind counts as invalid. */
export function parseQueryLog(text: string): { entries: QueryLogEntry[]; invalid: number } {
	const entries: QueryLogEntry[] = [];
	let invalid = 0;
	for (const line of text.split('\n')) {
		if (!line.trim()) continue;
		try {
			const v = JSON.parse(line);
			if (v && typeof v === 'object' && !Array.isArray(v) && typeof v.kind === 'string') entries.push(v);
			else invalid++;
		} catch {
			invalid++;
		}
	}
	return { entries, invalid };
}

export const entryText = (e: QueryLogEntry) => e.sql ?? e.command ?? '';

/** Keeps entries matching every set filter; the text matches the sql or command without case. */
export function filterQueryLog(entries: QueryLogEntry[], f: QueryLogFilter): QueryLogEntry[] {
	const text = f.text?.trim().toLowerCase();
	return entries.filter(
		(e) =>
			(!f.kind || e.kind === f.kind) &&
			(!f.decision || e.decision === f.decision) &&
			(!f.errorsOnly || !!e.error) &&
			(!text || entryText(e).toLowerCase().includes(text))
	);
}

/** The first `shown` entries and how many are left behind. */
export function queryLogPage(entries: QueryLogEntry[], shown: number): { visible: QueryLogEntry[]; rest: number } {
	return { visible: entries.slice(0, shown), rest: Math.max(0, entries.length - shown) };
}

/** The integrity banner for the X-Werbot-Recording-Verified header; none when the header is absent. */
export function verifiedBanner(header: string | null): BannerTone | undefined {
	if (header === 'true') return 'success';
	if (header === 'false') return 'alert';
	return undefined;
}

/** The distinct non-empty values of a field, sorted; they fill the filter menus. */
export const distinct = (entries: QueryLogEntry[], key: 'kind' | 'decision') => [...new Set(entries.map((e) => e[key]).filter((v): v is string => !!v))].sort();
