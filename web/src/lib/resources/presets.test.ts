import { expect, test } from 'vitest';
import type { ResourceType } from '#lib/gen/werbot/v1/resource_pb.js';
import { groupTypes, resourceTypeIcon, typeIcon } from './icons.js';
import { firstPort, launchHint } from './presets.js';

test('launchHint fills {host} and {port}, every occurrence', () => {
	expect(launchHint('psql -h {host} -p {port}', '127.0.0.1', 5432)).toBe('psql -h 127.0.0.1 -p 5432');
	expect(launchHint('{host}:{port} {host}', 'h', 1)).toBe('h:1 h');
});

test('launchHint without placeholders is returned as is, markup stays text', () => {
	expect(launchHint('redis-cli', '127.0.0.1', 6379)).toBe('redis-cli');
	expect(launchHint('<b>{host}</b>', 'h', 1)).toBe('<b>h</b>');
});

test('a preset sits in its category group next to builtin types', () => {
	const t = (name: string, category: string) => ({ name, category }) as ResourceType;
	const groups = groupTypes([t('ssh', 'servers'), t('tcp', 'servers'), t('postgres', 'databases')]);
	expect(groups.find((g) => g.category === 'databases')?.types.map((x) => x.name)).toEqual(['postgres']);
});

test('resourceTypeIcon uses a registry icon of the preset and falls back to its base type', () => {
	const t = (v: Partial<ResourceType>) => v as ResourceType;
	expect(resourceTypeIcon(t({ name: 'tcp', icon: '' }))).toBe(typeIcon('tcp'));
	expect(resourceTypeIcon(t({ name: 'pg', baseType: 'tcp', icon: 'storage' }))).toBe('storage');
	expect(resourceTypeIcon(t({ name: 'pg', baseType: 'udp', icon: 'no-such-icon' }))).toBe(typeIcon('udp'));
	expect(resourceTypeIcon(t({ name: 'pg', baseType: 'tcp', icon: '<img>' }))).toBe(typeIcon('tcp'));
});

test('firstPort reads the first range of a config and tolerates bad JSON', () => {
	expect(firstPort('{"host":"h","ports":[{"port":5432},{"port":8000,"endPort":8100}]}')).toBe(5432);
	expect(firstPort('{"host":"h"}')).toBe(0);
	expect(firstPort('nope')).toBe(0);
});
