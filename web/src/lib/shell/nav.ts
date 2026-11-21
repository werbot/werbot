import type { IconName } from '#lib/ui/index.js';
import * as m from '#lib/paraglide/messages.js';

export type SectionId = 'dashboard' | 'access' | 'organization' | 'profile';
export interface NavItem {
	href: string;
	label: () => string;
	icon: IconName;
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
	const access: NavItem[] = [
		...(manager ? [{ href: '/resources', label: m.nav_resources, icon: 'desktop' as const }] : []),
		{ href: '/sessions', label: m.nav_sessions, icon: 'history' },
		{ href: '/requests', label: m.nav_requests, icon: 'front_hand' },
		...(manager
			? [
					{ href: '/gateways', label: m.nav_gateways, icon: 'shield' as const },
					{ href: '/policies/approvals', label: m.nav_approval_policies, icon: 'filter_check' as const },
					{ href: '/policies/commands', label: m.nav_policies_commands, icon: 'editor/checklist' as const },
					{ href: '/policies/custom', label: m.nav_policies_custom, icon: 'editor/code' as const },
					{ href: '/policies/paths', label: m.nav_policies_paths, icon: 'folder-open' as const },
					{ href: '/policies/terminal', label: m.nav_policies_terminal, icon: 'keyboard' as const },
					{ href: '/locks', label: m.nav_locks, icon: 'lock' as const }
				]
			: [])
	];
	return [
		section('dashboard', m.nav_dashboard, 'grid_view', []),
		section('access', m.nav_access, 'desktop', access),
		...(manager
			? [
					section('organization', m.nav_organization, 'people', [
						{ href: '/members', label: m.nav_members, icon: 'people' },
						{ href: '/teams', label: m.nav_teams, icon: 'workspace' },
						{ href: '/audit', label: m.nav_audit, icon: 'report_detailed' },
						{ href: '/settings', label: m.nav_settings, icon: 'gear' }
					])
				]
			: []),
		section('profile', m.nav_profile, 'person', [
			{ href: '/profile', label: m.nav_profile, icon: 'person' },
			{ href: '/profile/keys', label: m.nav_keys, icon: 'key' },
			{ href: '/profile/cli-sessions', label: m.cli_sessions_title, icon: 'laptop' }
		])
	];
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
