import { expect, test } from 'vitest';
import { CLASSES, classLabel, classTone, verdictOf } from './classes';

test('class tones', () => {
	expect(classTone('destructive')).toBe('alert');
	expect(classTone('privilege')).toBe('alert');
	expect(classTone('read')).toBe('neutral');
	expect(classTone('write')).toBe('info');
	for (const c of ['egress', 'secrets', 'opaque']) expect(classTone(c)).toBe('warning');
	expect(CLASSES).toHaveLength(7);
});

test('unknown class is neutral and shown as sent', () => {
	expect(classTone('future')).toBe('neutral');
	expect(classLabel('future')).toBe('future');
	expect(classLabel('write')).toBe('write');
});

test('verdict', () => {
	expect(verdictOf({ allow: true, obligations: [] })).toBe('allowed');
	expect(verdictOf({ allow: false, obligations: [] })).toBe('denied');
	expect(verdictOf({ allow: false, obligations: ['approval'] })).toBe('approval');
});
