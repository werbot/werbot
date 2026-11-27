import { expect, test } from 'vitest';
import { resolveIcon } from '#lib/ui/icons.js';
import type { ResourceType } from '#lib/gen/werbot/v1/resource_pb.js';
import { categoryIcon, categoryLabel, groupTypes, typeIcon } from './icons.js';

const t = (name: string, category: string) => ({ name, category }) as ResourceType;

test('groupTypes keeps first-seen category order and skips empty categories', () => {
	const groups = groupTypes([t('ssh', 'servers'), t('pg', 'databases'), t('tcp', 'servers')]);
	expect(groups.map((g) => [g.category, g.types.map((x) => x.name)])).toEqual([
		['servers', ['ssh', 'tcp']],
		['databases', ['pg']]
	]);
	expect(groupTypes([])).toEqual([]);
});

test('every mapped icon exists and unknown names fall back', () => {
	for (const c of ['servers', 'databases', 'applications', 'desktops', 'containers', 'clouds']) {
		expect(resolveIcon(categoryIcon(c))).toBeDefined();
	}
	for (const n of ['ssh', 'tcp', 'telnet', 'udp', 'postgres']) expect(resolveIcon(typeIcon(n))).toBeDefined();
	expect(categoryIcon('nope')).toBe('powerplug');
	expect(typeIcon('nope')).toBe('powerplug');
	expect(resolveIcon('powerplug')).toBeDefined();
});

test('categoryLabel names known categories and falls back to the raw name', () => {
	expect(categoryLabel('servers')).toBe('Servers');
	expect(categoryLabel('mainframes')).toBe('mainframes');
});
