export type LabelRow = { key: string; value: string };

// "env=prod,team=a" -> { env: 'prod', team: 'a' }; malformed pairs are skipped.
export function parseLabelFilter(text: string): Record<string, string> {
	const out: Record<string, string> = {};
	for (const pair of text.split(',')) {
		const [key, ...rest] = pair.split('=');
		if (key.trim() && rest.length) out[key.trim()] = rest.join('=').trim();
	}
	return out;
}

export const rowsFromLabels = (labels: Record<string, string>): LabelRow[] =>
	Object.entries(labels).map(([key, value]) => ({ key, value }));

export const labelsFromRows = (rows: LabelRow[]): Record<string, string> =>
	Object.fromEntries(rows.filter((r) => r.key.trim()).map((r) => [r.key.trim(), r.value]));
