import { expect, test } from 'vitest';
import { resolveIcon } from '#lib/ui/icons.js';

// IconName is a plain string, so svelte-check does not catch an unknown icon:
// it renders as an empty slot. Every icon name used by the app must exist.
const sources = import.meta.glob(['/src/routes/**/*.svelte', '/src/lib/**/*.svelte', '!/src/lib/ui/**'], {
	query: '?raw',
	import: 'default',
	eager: true
}) as Record<string, string>;

test('every icon the app names exists in the icon set', () => {
	const missing: string[] = [];
	for (const [file, src] of Object.entries(sources)) {
		for (const [, name] of src.matchAll(/\bicon(?:: |=)['"]([a-z0-9_/-]+)['"]/g)) {
			if (!resolveIcon(name)) missing.push(`${file}: ${name}`);
		}
	}
	expect(Object.keys(sources).length).toBeGreaterThan(0);
	expect(missing).toEqual([]);
});
