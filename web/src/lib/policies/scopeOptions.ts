import type { SelectOption } from '#lib/ui/index.js';
import { members, resources, teams } from '#lib/api.js';
import { userOptions } from '#lib/members/options.js';

/** The users, teams and resources a policy rule can target, and an id to name map for the list cells. */
export interface ScopeOptions {
	users: SelectOption[];
	teams: SelectOption[];
	resources: SelectOption[];
	names: Map<string, string>;
}

export const noScope: ScopeOptions = { users: [], teams: [], resources: [], names: new Map() };

export async function loadScopeOptions(): Promise<ScopeOptions> {
	const [mem, tm, res] = await Promise.all([members.listMembers({}), teams.listTeams({}), resources.listResources({})]);
	const users = userOptions(mem.members);
	const teamOptions = tm.teams.map((t) => ({ value: t.id, label: t.name }));
	const resourceOptions = res.resources.map((x) => ({ value: x.id, label: x.name }));
	return {
		users,
		teams: teamOptions,
		resources: resourceOptions,
		names: new Map([...users, ...teamOptions, ...resourceOptions].map((o) => [o.value, o.label]))
	};
}
