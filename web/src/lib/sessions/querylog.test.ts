import { expect, test } from 'vitest';
import { distinct, filterQueryLog, parseQueryLog, queryLogPage, verifiedBanner, type QueryLogEntry } from './querylog';

const e = (o: Partial<QueryLogEntry>): QueryLogEntry => ({ t: 0, kind: 'query', ...o });

test('parseQueryLog skips blank and counts invalid', () => {
	const text = '{"t":0.1,"kind":"query","sql":"select 1"}\n\n{"t":0.2,"kind":"execute","sql":"x"}\n{oops\n[1]\n{"t":0.3,"kind":"query"}\n';
	const { entries, invalid } = parseQueryLog(text);
	expect(entries).toHaveLength(3);
	expect(invalid).toBe(2);
	expect(parseQueryLog('')).toEqual({ entries: [], invalid: 0 });
});

test('filterQueryLog by kind, decision, text, errors', () => {
	const all = [
		e({ kind: 'execute', sql: 'select 1', decision: 'allow' }),
		e({ kind: 'query', sql: 'DROP TABLE t', decision: 'deny', error: 'denied' }),
		e({ kind: 'command', command: 'ls' })
	];
	expect(filterQueryLog(all, { kind: 'execute' })).toHaveLength(1);
	expect(filterQueryLog(all, { decision: 'allow' })).toHaveLength(1);
	expect(filterQueryLog(all, { text: 'SELECT' })).toHaveLength(1);
	expect(filterQueryLog(all, { text: 'ls' })).toHaveLength(1);
	expect(filterQueryLog(all, { errorsOnly: true })).toHaveLength(1);
	expect(filterQueryLog(all, {})).toHaveLength(3);
});

test('queryLogPage shows the first 1000 and the rest on demand', () => {
	const all = Array.from({ length: 1500 }, () => e({}));
	expect(queryLogPage(all, 1000)).toMatchObject({ rest: 500 });
	expect(queryLogPage(all, 1000).visible).toHaveLength(1000);
	expect(queryLogPage(all, 2000)).toMatchObject({ rest: 0 });
});

test('verifiedBanner maps the header to a banner tone', () => {
	expect(verifiedBanner('true')).toBe('success');
	expect(verifiedBanner('false')).toBe('alert');
	expect(verifiedBanner(null)).toBeUndefined();
});

test('distinct lists sorted non-empty values', () => {
	expect(distinct([e({ kind: 'b' }), e({ kind: 'a' }), e({ kind: 'b', decision: 'allow' })], 'kind')).toEqual(['a', 'b']);
	expect(distinct([e({}), e({ decision: 'allow' })], 'decision')).toEqual(['allow']);
});
