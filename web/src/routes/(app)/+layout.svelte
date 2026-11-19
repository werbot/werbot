<script lang="ts">
		import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Avatar, Toaster } from '#lib/ui/index.js';
	import { auth } from '#lib/api.js';
	import { Role } from '#lib/gen/werbot/v1/user_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import { serverInfo } from '#lib/version.js';
	import { itemFor, navSections, sectionFor } from '#lib/shell/nav.js';
	import PrimaryMenu from '#lib/shell/PrimaryMenu.svelte';
	import ContextColumn from '#lib/shell/ContextColumn.svelte';
	import type { LayoutProps } from './$types';

	let { data, children }: LayoutProps = $props();

	let manager = $derived(data.session.role === Role.OWNER || data.session.role === Role.ADMIN);
	let sections = $derived(navSections(manager));
	let section = $derived(sectionFor(sections, page.url.pathname));
	let user = $derived({ name: data.session.user?.name ?? '', email: data.session.user?.email ?? '' });

	// DB-IP Lite requires the credit; other GeoIP databases do not.
	const info = serverInfo();
	let geoipAttribution = $derived(info.geoipAttribution);

	async function logout() {
		await auth.logout({});
		await goto('/login');
	}
</script>

{#snippet profileHeader()}
	<div class="profile wb-me">
		<Avatar name={user.name} size={64} />
		<span class="profile__name">{user.name}</span>
		<span class="profile__email">{user.email}</span>
	</div>
{/snippet}

{#snippet credit()}
	<a class="attribution" href="https://db-ip.com">{m.geoip_attribution()}</a>
{/snippet}

<div class={['shell', section.items.length === 0 && 'shell--bare']}>
	<PrimaryMenu {sections} active={section.id} {user} onlogout={logout} />
	{#if section.items.length > 0}
		<ContextColumn
			{section}
			current={itemFor(section, page.url.pathname)}
			header={section.id === 'profile' ? profileHeader : undefined}
			footer={geoipAttribution ? credit : undefined}
		/>
	{/if}
	<main class="miso-scroll">
		<div class="work">{@render children()}</div>
		{#if geoipAttribution}
			<!-- the credit must be on every page: here it stands in for the column footer -->
			<footer class="work__credit">{@render credit()}</footer>
		{/if}
	</main>
</div>
<Toaster />

<style>
	.shell {
		display: grid;
		grid-template-columns: 64px 1fr;
		height: 100dvh;
	}

	main {
		display: flex;
		flex-direction: column;
		min-width: 0;
		overflow: auto;
		background: var(--miso-cont-top);
	}

	/* the dashboard has no column and keeps the demo dashboard ground */
	.shell--bare main {
		background: var(--miso-cont-mid);
	}

	.work {
		flex: 1 0 auto;
	}

	.work__credit {
		display: flex;
		justify-content: center;
		padding: 16px;
	}

	/* desktop from 1024px: the context column shows and carries the credit; below it only the rail stays */
	@media (min-width: 1024px) {
		.shell:not(.shell--bare) {
			grid-template-columns: 64px 260px 1fr;
		}

		.shell:not(.shell--bare) .work__credit {
			display: none;
		}
	}

	.profile {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 2px;
		padding: 0 4px;
	}

	.profile :global(.miso-avatar) {
		margin-bottom: 10px;
		border-radius: 12px;
	}

	.profile__name {
		font: var(--miso-font-ui-16-semi);
		color: var(--miso-txt-accent);
	}

	.profile__email {
		font: var(--miso-font-ui-12-reg);
		color: var(--miso-txt-secondary);
	}

	/* the name cell of a list row: the link or button that opens the row */
	:global(.row-name) {
		font-weight: 600;
		background: none;
		border: 0;
		padding: 0;
		cursor: pointer;
		color: inherit;
		text-decoration: none;
	}

	:global(.row-name:focus-visible) {
		outline: 2px solid var(--miso-line-accent);
		outline-offset: 2px;
		border-radius: var(--miso-radius-04);
	}

	.attribution {
		font: var(--miso-font-ui-11-reg);
		color: var(--miso-txt-tertiary);
		text-decoration: none;
		white-space: nowrap;
	}

	.attribution:hover {
		color: var(--miso-txt-primary);
	}
</style>
