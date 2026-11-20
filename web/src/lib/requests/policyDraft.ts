import type { MessageInitShape } from '@bufbuild/protobuf';
import type { ApprovalPolicy, CreateApprovalPolicyRequest, CreateApprovalPolicyRequestSchema } from '#lib/gen/werbot/v1/approval_policy_pb.js';
import { formatDuration, parseDuration } from '#lib/duration.js';
import { labelsFromRows, rowsFromLabels, type LabelRow } from '#lib/resources/labels.js';

/** The editable form of an approval policy: plain arrays and strings the form binds to. */
export interface PolicyDraft {
	name: string;
	/** 'resources' or 'labels': which selector the policy uses */
	by: string;
	resourceIds: string[];
	labels: LabelRow[];
	requesters: { users: string[]; teams: string[]; allUsers: boolean; allAgents: boolean };
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
	const labels = p?.resourceSelector?.labels ?? {};
	return {
		name: p?.name ?? '',
		by: Object.keys(labels).length > 0 ? 'labels' : 'resources',
		resourceIds: [...(p?.resourceSelector?.resourceIds ?? [])],
		labels: rowsFromLabels(labels),
		requesters: {
			users: [...(p?.requesters?.users ?? [])],
			teams: [...(p?.requesters?.teams ?? [])],
			allUsers: p?.requesters?.allUsers ?? false,
			allAgents: p?.requesters?.allAgents ?? false
		},
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
	const byResources = d.by === 'resources';
	return {
		name: d.name,
		resourceSelector: { resourceIds: byResources ? [...d.resourceIds] : [], labels: byResources ? {} : labelsFromRows(d.labels) },
		requesters: { ...d.requesters, users: [...d.requesters.users], teams: [...d.requesters.teams] },
		approvers: { users: [...d.approvers.users], teams: [...d.approvers.teams], allUsers: false, allAgents: false },
		mode: d.mode,
		maxDuration,
		requireReason: d.requireReason,
		fourEyes: d.fourEyes,
		breakGlass: d.breakGlass
	};
}
