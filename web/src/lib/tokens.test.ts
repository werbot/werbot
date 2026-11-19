import { readFileSync } from 'node:fs';
import { expect, test } from 'vitest';

// A misspelt token (--miso-sp-8 instead of --miso-sp-08) makes the whole declaration invalid
// and the browser drops it silently. Every --miso-* token the app reads must be defined.
const app = import.meta.glob(['/src/routes/**/*.svelte', '/src/lib/**/*.svelte', '!/src/lib/ui/**'], { query: '?raw', import: 'default', eager: true }) as Record<string, string>;
// CSS files come back empty through import.meta.glob under vitest, so read them from disk
const css = ['src/lib/ui/styles/tokens.css', 'src/lib/ui/styles/base.css', 'src/lib/theme.css'].map((f) => readFileSync(f, 'utf8'));
const lib = import.meta.glob(['/src/lib/ui/**/*.svelte'], { query: '?raw', import: 'default', eager: true }) as Record<string, string>;

test('every --miso-* token the app reads is defined', () => {
	const defined = new Set<string>();
	for (const src of [...css, ...Object.values(lib), ...Object.values(app)]) {
		for (const [, name] of src.matchAll(/(--miso-[a-z0-9-]+)\s*:/g)) defined.add(name);
	}
	const missing: string[] = [];
	for (const [file, src] of Object.entries(app)) {
		for (const [, name] of src.matchAll(/var\((--miso-[a-z0-9-]+)/g)) {
			if (!defined.has(name)) missing.push(`${file}: ${name}`);
		}
	}
	expect(Object.keys(app).length).toBeGreaterThan(0);
	expect(missing).toEqual([]);
});
