import { expect, test } from 'vitest';
import { CLASSES, classLabel, classColor, verdictOf } from './classes';

test('class colors', () => {
	expect(classColor('destructive')).toBe('red');
	expect(classColor('privilege')).toBe('red');
	expect(classColor('read')).toBe('coolgrey');
	expect(classColor('write')).toBe('blue');
	for (const c of ['egress', 'secrets', 'opaque']) expect(classColor(c)).toBe('orange');
	expect(CLASSES).toHaveLength(7);
});

test('unknown class is neutral and shown as sent', () => {
	expect(classColor('future')).toBe('coolgrey');
	expect(classLabel('future')).toBe('future');
	expect(classLabel('write')).toBe('write');
});

test('verdict', () => {
	expect(verdictOf({ allow: true, obligations: [] })).toBe('allowed');
	expect(verdictOf({ allow: false, obligations: [] })).toBe('denied');
	expect(verdictOf({ allow: false, obligations: ['approval'] })).toBe('approval');
});
