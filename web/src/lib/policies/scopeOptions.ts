import type { SelectOption } from '#lib/ui/index.js';
import { members, resources, teams } from '#lib/api.js';
import { userOptions } from '#lib/members/options.js';
import { hasQuery } from '#lib/resources/agentCredential.js';

/** The users, teams and resources a policy rule can target, an id to name map for the list cells, and the resources that take SQL. */
export interface ScopeOptions {
	users: SelectOption[];
	teams: SelectOption[];
	resources: SelectOption[];
	names: Map<string, string>;
	queryResources: Set<string>;
}

export const noScope: ScopeOptions = { users: [], teams: [], resources: [], names: new Map(), queryResources: new Set() };

export async function loadScopeOptions(): Promise<ScopeOptions> {
	const [mem, tm, res, rt] = await Promise.all([
		members.listMembers({}),
		teams.listTeams({}),
		resources.listResources({}),
		resources.listResourceTypes({})
	]);
	const users = userOptions(mem.members);
	const teamOptions = tm.teams.map((t) => ({ value: t.id, label: t.name }));
	const resourceOptions = res.resources.map((x) => ({ value: x.id, label: x.name }));
	return {
		users,
		teams: teamOptions,
		resources: resourceOptions,
		names: new Map([...users, ...teamOptions, ...resourceOptions].map((o) => [o.value, o.label])),
		queryResources: new Set(res.resources.filter((x) => hasQuery(rt.types, x.type)).map((x) => x.id))
	};
}
