import type { ResourceSelector, Subjects } from '#lib/gen/werbot/v1/selector_pb.js';
import { labelsFromRows, rowsFromLabels, type LabelRow } from './labels.js';

/** The resource selector as a form edits it: one of the two ways is sent. */
export interface SelectorDraft {
	/** 'resources' or 'labels': which selector is used */
	by: string;
	resourceIds: string[];
	labels: LabelRow[];
}

export interface SubjectsDraft {
	users: string[];
	teams: string[];
	allUsers: boolean;
	allAgents: boolean;
}

export function toSelectorDraft(s?: ResourceSelector): SelectorDraft {
	const labels = s?.labels ?? {};
	return { by: Object.keys(labels).length > 0 ? 'labels' : 'resources', resourceIds: [...(s?.resourceIds ?? [])], labels: rowsFromLabels(labels) };
}

export function fromSelectorDraft(d: SelectorDraft) {
	const byResources = d.by === 'resources';
	return { resourceIds: byResources ? [...d.resourceIds] : [], labels: byResources ? {} : labelsFromRows(d.labels) };
}

export const toSubjectsDraft = (s?: Subjects): SubjectsDraft => ({
	users: [...(s?.users ?? [])],
	teams: [...(s?.teams ?? [])],
	allUsers: s?.allUsers ?? false,
	allAgents: s?.allAgents ?? false
});

export const fromSubjectsDraft = (s: SubjectsDraft) => ({ ...s, users: [...s.users], teams: [...s.teams] });
