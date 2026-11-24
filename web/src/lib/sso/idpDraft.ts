import type { MessageInitShape } from '@bufbuild/protobuf';
import type { ClaimMapping, CreateIdentityProviderRequestSchema, IdentityProvider } from '#lib/gen/werbot/v1/identity_provider_pb.js';

/** A claim mapping row as the editor shows it: the target is a team id or a role. */
export interface MappingRow {
	claim: string;
	regex: string;
	/** "team" | "role" */
	kind: string;
	target: string;
}

export interface IdpDraft {
	name: string;
	issuer: string;
	clientId: string;
	/** write only: empty keeps the stored secret */
	clientSecret: string;
	/** space or comma separated */
	scopes: string;
	mapping: MappingRow[];
	allowSignup: boolean;
	enabled: boolean;
}

export type IdpInput = Exclude<MessageInitShape<typeof CreateIdentityProviderRequestSchema>, { $typeName: string }>;

const DEFAULT_SCOPES = 'openid email profile groups';

const rowOf = (c: ClaimMapping): MappingRow => ({
	claim: c.claim,
	regex: c.regex,
	kind: c.target.case === 'teamId' ? 'team' : 'role',
	target: c.target.value ?? ''
});

export function toDraft(p?: IdentityProvider): IdpDraft {
	return {
		name: p?.name ?? '',
		issuer: p?.issuer ?? '',
		clientId: p?.clientId ?? '',
		clientSecret: '',
		scopes: p ? p.scopes.join(' ') : DEFAULT_SCOPES,
		mapping: (p?.claimsMapping ?? []).map(rowOf),
		allowSignup: p?.allowSignup ?? false,
		enabled: p?.enabled ?? true
	};
}

/** A row needs a claim, a regex the browser can compile and a target. */
export function mappingInvalid(r: MappingRow): boolean {
	if (!r.claim.trim() || !r.regex || !r.target) return true;
	try {
		new RegExp(r.regex);
		return false;
	} catch {
		return true;
	}
}

export function fromDraft(d: IdpDraft): IdpInput {
	return {
		name: d.name,
		issuer: d.issuer,
		clientId: d.clientId,
		clientSecret: d.clientSecret,
		scopes: d.scopes.split(/[\s,]+/).filter(Boolean),
		claimsMapping: d.mapping.map((r) => ({
			claim: r.claim.trim(),
			regex: r.regex,
			target: r.kind === 'team' ? { case: 'teamId' as const, value: r.target } : { case: 'role' as const, value: r.target }
		})),
		allowSignup: d.allowSignup,
		enabled: d.enabled
	};
}
