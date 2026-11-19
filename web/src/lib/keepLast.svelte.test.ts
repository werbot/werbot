import { flushSync } from 'svelte';
import { expect, test } from 'vitest';
import { keepLast } from './keepLast.svelte.js';

test('keeps the last non-empty value', () => {
	let src = $state<string | null>(null);
	const cleanup = $effect.root(() => {
		const shown = keepLast(() => src);
		flushSync();
		expect(shown.value).toBeNull();
		src = 'A';
		flushSync();
		expect(shown.value).toBe('A');
		src = null;
		flushSync();
		expect(shown.value).toBe('A');
		src = 'B';
		flushSync();
		expect(shown.value).toBe('B');
	});
	cleanup();
});
