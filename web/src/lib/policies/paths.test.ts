import { describe, expect, it } from 'vitest';
import { applyTemplate, fromPathDraft, parsePaths, PATH_TEMPLATES, toPathDraft } from './paths.js';

describe('path rule draft', () => {
	it('lists the path templates', () => {
		expect(PATH_TEMPLATES.map((t) => t.key)).toEqual(['work_folder', 'deny_folder', 'read_logs']);
		expect(PATH_TEMPLATES[0]).toMatchObject({ paths: ['~/app/**'], access: 'any', effect: 'only' });
	});
	it('parses one glob per line', () => {
		expect(parsePaths(' /a/**\n\n~/b\n/a/** ')).toEqual(['/a/**', '~/b']);
	});
	it('a template keeps name and scope', () => {
		const d = { ...toPathDraft(), name: 'work' };
		expect(applyTemplate(d, PATH_TEMPLATES[1])).toMatchObject({ name: 'work', paths: '/etc/**', access: 'write', effect: 'deny' });
	});
	it('builds the request from the draft', () => {
		const d = { ...toPathDraft(), name: 'n', paths: ' ~/a \n~/a\n/b', resourceIds: ['r1'] };
		expect(fromPathDraft(d)).toMatchObject({ name: 'n', paths: ['~/a', '/b'], access: 'any', effect: 'only', resourceSelector: { resourceIds: ['r1'] } });
	});
});
