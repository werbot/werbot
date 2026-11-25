import { expect, test } from 'vitest';
import { groupHeading, itemFor, navSections, sectionFor } from './nav';

const hrefs = (s: { items: { href: string }[] }) => s.items.map((i) => i.href);
const byId = (manager: boolean, id: string) => navSections(manager).find((s) => s.id === id)!;

test('member sections', () => {
	const s = navSections(false);
	expect(s.map((x) => x.id)).toEqual(['dashboard', 'access', 'profile']);
	expect(hrefs(s[1])).toEqual(['/sessions', '/requests', '/agents']);
	expect(s.flatMap(hrefs)).not.toContain('/settings');
	expect(hrefs(s[2])).toEqual(['/profile', '/profile/keys', '/profile/mfa', '/profile/cli-sessions']);
});

test('manager sections', () => {
	expect(hrefs(byId(true, 'access'))).toEqual(['/resources', '/resources/presets', '/gateways', '/sessions', '/requests', '/agents', '/locks', '/policies/approvals', '/policies/commands', '/policies/paths', '/policies/terminal', '/policies/mfa', '/policies/custom']);
	expect(hrefs(byId(true, 'organization'))).toEqual(['/members', '/teams', '/settings/sso', '/audit', '/settings']);
	expect(hrefs(byId(true, 'profile'))).toEqual(['/profile', '/profile/keys', '/profile/mfa', '/profile/cli-sessions']);
});

test('group headings', () => {
	const headings = (items: Parameters<typeof groupHeading>[0]) => items.map((_, i) => groupHeading(items, i)).filter(Boolean);
	expect(headings(byId(true, 'access').items)).toEqual(['Infrastructure', 'Activity', 'Policies']);
	expect(headings(byId(true, 'organization').items)).toEqual(['People', 'Security', 'General']);
	// one group alone gets no heading
	expect(headings(byId(false, 'access').items)).toEqual([]);
	expect(headings(byId(true, 'profile').items)).toEqual([]);
});

test('dashboard has no column', () => {
	const d = sectionFor(navSections(true), '/');
	expect(d.id).toBe('dashboard');
	expect(d.items.length).toBe(0);
});

test('nested paths', () => {
	const s = navSections(true);
	expect(sectionFor(s, '/resources/7f0c-1').id).toBe('access');
	expect(sectionFor(s, '/profile/delete/tok').id).toBe('profile');
	const profile = byId(true, 'profile');
	expect(itemFor(profile, '/profile/keys')?.href).toBe('/profile/keys');
	expect(itemFor(profile, '/profile/mfa')?.href).toBe('/profile/mfa');
	expect(itemFor(byId(true, 'access'), '/resources/presets')?.href).toBe('/resources/presets');
	expect(itemFor(byId(true, 'access'), '/resources/7f0c-1')?.href).toBe('/resources');
	expect(itemFor(byId(true, 'organization'), '/settings/sso')?.href).toBe('/settings/sso');
	expect(itemFor(profile, '/profile/delete/tok')?.href).toBe('/profile');
});

test('unknown path', () => {
	expect(sectionFor(navSections(true), '/nope').id).toBe('dashboard');
});
