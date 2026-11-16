<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Sidebar, SidebarItem } from '#lib/ui/index.js';
	import { auth } from '#lib/api.js';
	import { Role } from '#lib/gen/werbot/v1/user_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import type { LayoutProps } from './$types';

	let { data, children }: LayoutProps = $props();

	let manager = $derived(data.session.role === Role.OWNER || data.session.role === Role.ADMIN);
	let items = $derived([
		{ href: '/', icon: 'dashboard', label: m.nav_dashboard() },
		{ href: '/profile', icon: 'user', label: m.nav_profile() },
		{ href: '/profile/keys', icon: 'key', label: m.nav_keys() },
		...(manager
			? [
					{ href: '/members', icon: 'users', label: m.nav_members() },
					{ href: '/teams', icon: 'workspace', label: m.nav_teams() }
				]
			: [])
	]);

	async function logout() {
		await auth.logout({});
		await goto('/login');
	}
</script>

<div class="shell">
	<Sidebar expanded>
		{#each items as item (item.href)}
			<SidebarItem icon={item.icon} active={page.url.pathname === item.href} onclick={() => goto(item.href)}>
				{item.label}
			</SidebarItem>
		{/each}
		{#snippet footer()}
			<SidebarItem icon="logout" onclick={logout}>{m.auth_logout()}</SidebarItem>
		{/snippet}
	</Sidebar>
	<main>{@render children()}</main>
</div>

<style>
	.shell {
		display: flex;
		height: 100vh;
	}

	main {
		flex: 1;
		min-width: 0;
		padding: 24px;
		overflow: auto;
	}
</style>
