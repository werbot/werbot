import type { CommandRule, CreateCommandRuleRequest, CreateCommandRuleRequestSchema } from '#lib/gen/werbot/v1/policy_pb.js';
import type { MessageInitShape } from '@bufbuild/protobuf';
import { fromSelectorDraft, fromSubjectsDraft, toSelectorDraft, toSubjectsDraft, type SelectorDraft, type SubjectsDraft } from '#lib/resources/selectorDraft.js';

/** The editable form of a command rule: plain arrays and strings the form binds to. */
export interface RuleDraft extends SelectorDraft {
	name: string;
	subjects: SubjectsDraft;
	classes: string[];
	/** "allow" | "deny" | "approval" */
	effect: string;
	message: string;
}

/** the plain init object (no $typeName), so it also fills an update request */
export type RuleInput = Exclude<MessageInitShape<typeof CreateCommandRuleRequestSchema>, CreateCommandRuleRequest>;

export function toDraft(r?: CommandRule): RuleDraft {
	return {
		name: r?.name ?? '',
		subjects: toSubjectsDraft(r?.subjects),
		...toSelectorDraft(r?.resourceSelector),
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
	return {
		name: d.name,
		subjects: fromSubjectsDraft(d.subjects),
		resourceSelector: fromSelectorDraft(d),
		classes: [...d.classes],
		effect: d.effect,
		message: d.message
	};
}
