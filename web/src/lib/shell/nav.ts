import type { IconName } from '#lib/ui/index.js';
import * as m from '#lib/paraglide/messages.js';

export type SectionId = 'dashboard' | 'access' | 'organization' | 'profile';
export interface NavItem {
	href: string;
	label: () => string;
	icon: IconName;
	/** the subheading the item sits under in the context column */
	group?: () => string;
}
export interface NavSection {
	id: SectionId;
	label: () => string;
	icon: IconName;
	href: string;
	items: NavItem[];
}

const section = (id: SectionId, label: () => string, icon: IconName, items: NavItem[]): NavSection => ({
	id,
	label,
	icon,
	href: items[0]?.href ?? '/',
	items
});

export function navSections(manager: boolean): NavSection[] {
	const infra = m.nav_group_infrastructure;
	const activity = m.nav_group_activity;
	const policies = m.nav_group_policies;
	const access: NavItem[] = [
		...(manager
			? [
					{ href: '/resources', label: m.nav_resources, icon: 'desktop' as const, group: infra },
					{ href: '/resources/presets', label: m.nav_presets, icon: 'storage' as const, group: infra },
					{ href: '/gateways', label: m.nav_gateways, icon: 'hub' as const, group: infra }
				]
			: []),
		{ href: '/sessions', label: m.nav_sessions, icon: 'history', group: activity },
		{ href: '/requests', label: m.nav_requests, icon: 'front_hand', group: activity },
		{ href: '/agents', label: m.nav_agents, icon: 'sparkle', group: activity },
		...(manager
			? [
					{ href: '/locks', label: m.nav_locks, icon: 'lock' as const, group: activity },
					{ href: '/policies/approvals', label: m.nav_approval_policies, icon: 'filter_check' as const, group: policies },
					{ href: '/policies/commands', label: m.nav_policies_commands, icon: 'editor/checklist' as const, group: policies },
					{ href: '/policies/paths', label: m.nav_policies_paths, icon: 'folder-open' as const, group: policies },
					{ href: '/policies/terminal', label: m.nav_policies_terminal, icon: 'keyboard' as const, group: policies },
					{ href: '/policies/mfa', label: m.nav_policies_mfa, icon: 'shield' as const, group: policies },
					{ href: '/policies/custom', label: m.nav_policies_custom, icon: 'editor/code' as const, group: policies }
				]
			: [])
	];
	return [
		section('dashboard', m.nav_dashboard, 'grid_view', []),
		section('access', m.nav_access, 'desktop', access),
		...(manager
			? [
					section('organization', m.nav_organization, 'people', [
						{ href: '/members', label: m.nav_members, icon: 'people', group: m.nav_group_people },
						{ href: '/teams', label: m.nav_teams, icon: 'workspace', group: m.nav_group_people },
						{ href: '/settings/sso', label: m.nav_sso, icon: 'key', group: m.nav_group_security },
						{ href: '/audit', label: m.nav_audit, icon: 'report_detailed', group: m.nav_group_security },
						{ href: '/settings', label: m.nav_settings, icon: 'gear', group: m.nav_group_general }
					])
				]
			: []),
		section('profile', m.nav_profile, 'person', [
			{ href: '/profile', label: m.nav_profile, icon: 'person' },
			{ href: '/profile/keys', label: m.nav_keys, icon: 'key' },
			{ href: '/profile/mfa', label: m.nav_mfa, icon: 'security' },
			{ href: '/profile/cli-sessions', label: m.cli_sessions_title, icon: 'laptop' }
		])
	];
}

/** the group subheading to show before item i: only where the group changes, and only in a column with more than one group */
export function groupHeading(items: NavItem[], i: number): string {
	const label = items[i].group?.();
	const groups = new Set(items.map((it) => it.group?.()));
	if (!label || groups.size < 2) return '';
	return i === 0 || items[i - 1].group?.() !== label ? label : '';
}

const matches = (pathname: string, href: string) => pathname === href || pathname.startsWith(href + '/');

export function itemFor(section: NavSection, pathname: string): NavItem | undefined {
	return section.items
		.filter((i) => matches(pathname, i.href))
		.sort((a, b) => b.href.length - a.href.length)[0];
}

export function sectionFor(sections: NavSection[], pathname: string): NavSection {
	return sections.find((s) => itemFor(s, pathname)) ?? sections[0];
}
