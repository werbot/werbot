import type { MessageInitShape } from '@bufbuild/protobuf';
import type { CreateMFARuleRequest, CreateMFARuleRequestSchema, MFARule } from '#lib/gen/werbot/v1/policy_mfa_pb.js';
import { formatDuration, parseDuration } from '#lib/duration.js';
import { fromSelectorDraft, fromSubjectsDraft, toSelectorDraft, toSubjectsDraft, type SelectorDraft, type SubjectsDraft } from '#lib/resources/selectorDraft.js';

/** The editable form of an MFA rule. */
export interface MfaRuleDraft extends SelectorDraft {
	name: string;
	subjects: SubjectsDraft;
	/** text such as "60s", "1h" */
	maxAge: string;
	/** "any" | "webauthn" */
	strength: string;
}

/** the plain init object (no $typeName): it also fills an update request */
export type MfaRuleInput = Exclude<MessageInitShape<typeof CreateMFARuleRequestSchema>, CreateMFARuleRequest>;

const MIN_AGE = 30;
const MAX_AGE = 86400;

export function toDraft(r?: MFARule): MfaRuleDraft {
	return {
		name: r?.name ?? '',
		subjects: toSubjectsDraft(r?.subjects),
		...toSelectorDraft(r?.resourceSelector),
		maxAge: r?.maxAge ? formatDuration(r.maxAge) : '60s',
		strength: r?.strength || 'any'
	};
}

/** The server accepts 30 s to 24 h. */
export function maxAgeInvalid(text: string): boolean {
	const d = parseDuration(text);
	return !d || d.seconds < MIN_AGE || d.seconds > MAX_AGE;
}

export function fromDraft(d: MfaRuleDraft): MfaRuleInput {
	return {
		name: d.name,
		subjects: fromSubjectsDraft(d.subjects),
		resourceSelector: fromSelectorDraft(d),
		maxAge: parseDuration(d.maxAge) ?? undefined,
		strength: d.strength
	};
}
