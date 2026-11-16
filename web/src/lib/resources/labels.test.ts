import { expect, test } from 'vitest';
import { labelsFromRows, parseLabelFilter } from './labels';

test('label filter parses key=value pairs and skips malformed ones', () => {
	expect(parseLabelFilter('env=prod, team=a=b,bad,=x')).toEqual({ env: 'prod', team: 'a=b' });
	expect(parseLabelFilter('')).toEqual({});
});

test('rows drop empty keys', () => {
	expect(labelsFromRows([{ key: ' a ', value: '1' }, { key: '', value: '2' }])).toEqual({ a: '1' });
});
