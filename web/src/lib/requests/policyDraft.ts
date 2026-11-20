import type { MessageInitShape } from '@bufbuild/protobuf';
import type { ApprovalPolicy, CreateApprovalPolicyRequest, CreateApprovalPolicyRequestSchema } from '#lib/gen/werbot/v1/approval_policy_pb.js';
import { formatDuration, parseDuration } from '#lib/duration.js';
import { fromSelectorDraft, fromSubjectsDraft, toSelectorDraft, toSubjectsDraft, type SelectorDraft, type SubjectsDraft } from '#lib/resources/selectorDraft.js';

/** The editable form of an approval policy: plain arrays and strings the form binds to. */
export interface PolicyDraft extends SelectorDraft {
	name: string;
	requesters: SubjectsDraft;
	approvers: { users: string[]; teams: string[] };
	mode: string;
	/** text duration ("8h"); empty: the server default */
	maxDuration: string;
	requireReason: boolean;
	fourEyes: boolean;
	breakGlass: boolean;
}

/** the plain init object (no $typeName), so it also fills an update request */
export type PolicyInput = Exclude<MessageInitShape<typeof CreateApprovalPolicyRequestSchema>, CreateApprovalPolicyRequest>;

export function toDraft(p?: ApprovalPolicy): PolicyDraft {
	return {
		name: p?.name ?? '',
		...toSelectorDraft(p?.resourceSelector),
		requesters: toSubjectsDraft(p?.requesters),
		approvers: { users: [...(p?.approvers?.users ?? [])], teams: [...(p?.approvers?.teams ?? [])] },
		mode: p?.mode || 'manual',
		maxDuration: formatDuration(p?.maxDuration),
		requireReason: p?.requireReason ?? true,
		fourEyes: p?.fourEyes ?? false,
		breakGlass: p?.breakGlass ?? false
	};
}

/** The request fields of a draft; null when the maximum duration does not parse. */
export function fromDraft(d: PolicyDraft): PolicyInput | null {
	const maxDuration = d.maxDuration.trim() ? parseDuration(d.maxDuration) : undefined;
	if (maxDuration === null) return null;
	return {
		name: d.name,
		resourceSelector: fromSelectorDraft(d),
		requesters: fromSubjectsDraft(d.requesters),
		approvers: { users: [...d.approvers.users], teams: [...d.approvers.teams], allUsers: false, allAgents: false },
		mode: d.mode,
		maxDuration,
		requireReason: d.requireReason,
		fourEyes: d.fourEyes,
		breakGlass: d.breakGlass
	};
}
