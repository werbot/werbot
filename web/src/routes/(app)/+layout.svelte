<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Sidebar, SidebarItem, Toaster } from '#lib/ui/index.js';
	import { auth, system } from '#lib/api.js';
	import { Role } from '#lib/gen/werbot/v1/user_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import type { LayoutProps } from './$types';

	let { data, children }: LayoutProps = $props();

	let manager = $derived(data.session.role === Role.OWNER || data.session.role === Role.ADMIN);
	let items = $derived([
		{ href: '/', icon: 'dashboard', label: m.nav_dashboard() },
		{ href: '/profile', icon: 'user', label: m.nav_profile() },
		{ href: '/profile/keys', icon: 'key', label: m.nav_keys() },
		{ href: '/sessions', icon: 'history', label: m.nav_sessions() },
		...(manager
			? [
					{ href: '/members', icon: 'users', label: m.nav_members() },
					{ href: '/teams', icon: 'workspace', label: m.nav_teams() },
					{ href: '/resources', icon: 'desktop', label: m.nav_resources() },
					{ href: '/gateways', icon: 'shield', label: m.nav_gateways() },
					{ href: '/audit', icon: 'report_detailed', label: m.nav_audit() }
				]
			: [])
	]);

	// DB-IP Lite requires the credit; other GeoIP databases do not.
	let geoipAttribution = $state(false);

	onMount(() => {
		system
			.getVersion({})
			.then((res) => (geoipAttribution = res.geoipAttribution))
			.catch(() => {});
	});

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
			{#if geoipAttribution}
				<a class="attribution" href="https://db-ip.com">{m.geoip_attribution()}</a>
			{/if}
		{/snippet}
	</Sidebar>
	<main>
		{@render children()}
	</main>
</div>
<Toaster />

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

	.attribution {
		padding: 4px 10px 0;
		font: var(--miso-font-ui-11-reg);
		color: var(--miso-sidebar-ico-base);
		text-decoration: none;
		white-space: nowrap;
	}

	.attribution:hover {
		color: var(--miso-sidebar-ico-hover);
	}
</style>
