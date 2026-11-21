import type { MessageInitShape } from '@bufbuild/protobuf';
import type { CreatePathRuleRequest, CreatePathRuleRequestSchema, PathRule } from '#lib/gen/werbot/v1/policy_paths_pb.js';
import { fromSelectorDraft, fromSubjectsDraft, toSelectorDraft, toSubjectsDraft, type SelectorDraft, type SubjectsDraft } from '#lib/resources/selectorDraft.js';

export const ACCESS = ['read', 'write', 'any'] as const;
export const PATH_EFFECTS = ['only', 'deny', 'approval'] as const;

export type PathTemplate = {
	key: 'work_folder' | 'deny_folder' | 'read_logs';
	paths: string[];
	access: (typeof ACCESS)[number];
	effect: (typeof PATH_EFFECTS)[number];
};

export const PATH_TEMPLATES: PathTemplate[] = [
	{ key: 'work_folder', paths: ['~/app/**'], access: 'any', effect: 'only' },
	{ key: 'deny_folder', paths: ['/etc/**'], access: 'write', effect: 'deny' },
	{ key: 'read_logs', paths: ['/var/log/**'], access: 'read', effect: 'only' }
];

/** One glob per line: trimmed, no empty lines, no repeats, order kept. */
export const parsePaths = (text: string): string[] => [...new Set(text.split('\n').map((l) => l.trim()).filter(Boolean))];

/** The editable form of a path rule: the folders are one text, a glob per line. */
export interface PathDraft extends SelectorDraft {
	name: string;
	subjects: SubjectsDraft;
	paths: string;
	access: string;
	effect: string;
	message: string;
}

/** the plain init object (no $typeName), so it also fills an update request */
export type PathRuleInput = Exclude<MessageInitShape<typeof CreatePathRuleRequestSchema>, CreatePathRuleRequest>;

export const toPathDraft = (r?: PathRule): PathDraft => ({
	name: r?.name ?? '',
	subjects: toSubjectsDraft(r?.subjects),
	...toSelectorDraft(r?.resourceSelector),
	paths: (r?.paths ?? []).join('\n'),
	access: r?.access || 'any',
	effect: r?.effect || 'only',
	message: r?.message ?? ''
});

/** A template fills folders, access and effect of a draft and keeps the rest. */
export const applyTemplate = (d: PathDraft, t: PathTemplate): PathDraft => ({ ...d, paths: t.paths.join('\n'), access: t.access, effect: t.effect });

export const fromPathDraft = (d: PathDraft): PathRuleInput => ({
	name: d.name,
	subjects: fromSubjectsDraft(d.subjects),
	resourceSelector: fromSelectorDraft(d),
	paths: parsePaths(d.paths),
	access: d.access,
	effect: d.effect,
	message: d.message
});
