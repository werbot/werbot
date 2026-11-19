import { expect, test } from 'vitest';
import { safeNext } from './next';

test('safeNext keeps a root path with its query', () => {
	expect(safeNext('/cli/authorize?id=1')).toBe('/cli/authorize?id=1');
	expect(safeNext('/')).toBe('/');
});

test('safeNext turns anything that leaves the origin into /', () => {
	for (const s of ['//evil.com', '/\\evil.com', 'https://evil.com', 'evil.com', '/\tevil', '', null]) {
		expect(safeNext(s), String(s)).toBe('/');
	}
});
