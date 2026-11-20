import type { CommandRule, CreateCommandRuleRequest, CreateCommandRuleRequestSchema } from '#lib/gen/werbot/v1/policy_pb.js';
import type { MessageInitShape } from '@bufbuild/protobuf';
import { labelsFromRows, rowsFromLabels, type LabelRow } from '#lib/resources/labels.js';

/** The editable form of a command rule: plain arrays and strings the form binds to. */
export interface RuleDraft {
	name: string;
	subjects: { users: string[]; teams: string[]; allUsers: boolean; allAgents: boolean };
	/** 'resources' or 'labels': which selector the rule uses */
	by: string;
	resourceIds: string[];
	labels: LabelRow[];
	classes: string[];
	/** "allow" | "deny" | "approval" */
	effect: string;
	message: string;
}

/** the plain init object (no $typeName), so it also fills an update request */
export type RuleInput = Exclude<MessageInitShape<typeof CreateCommandRuleRequestSchema>, CreateCommandRuleRequest>;

export function toDraft(r?: CommandRule): RuleDraft {
	const labels = r?.resourceSelector?.labels ?? {};
	return {
		name: r?.name ?? '',
		subjects: {
			users: [...(r?.subjects?.users ?? [])],
			teams: [...(r?.subjects?.teams ?? [])],
			allUsers: r?.subjects?.allUsers ?? false,
			allAgents: r?.subjects?.allAgents ?? false
		},
		by: Object.keys(labels).length > 0 ? 'labels' : 'resources',
		resourceIds: [...(r?.resourceSelector?.resourceIds ?? [])],
		labels: rowsFromLabels(labels),
		classes: [...(r?.classes ?? [])],
		effect: r?.effect || 'deny',
		message: r?.message ?? ''
	};
}

/** The empty-state shortcut: every user needs an approval for commands that cannot be classified. */
export const presetDraft = (): RuleDraft => ({
	...toDraft(),
	name: 'opaque-approval',
	subjects: { users: [], teams: [], allUsers: true, allAgents: false },
	classes: ['opaque'],
	effect: 'approval'
});

export function fromDraft(d: RuleDraft): RuleInput {
	const byResources = d.by === 'resources';
	return {
		name: d.name,
		subjects: { ...d.subjects, users: [...d.subjects.users], teams: [...d.subjects.teams] },
		resourceSelector: { resourceIds: byResources ? [...d.resourceIds] : [], labels: byResources ? {} : labelsFromRows(d.labels) },
		classes: [...d.classes],
		effect: d.effect,
		message: d.message
	};
}
