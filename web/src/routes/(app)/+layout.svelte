<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Avatar, Toaster } from '#lib/ui/index.js';
	import { auth } from '#lib/api.js';
	import { Role } from '#lib/gen/werbot/v1/user_pb.js';
	import { itemFor, navSections, sectionFor } from '#lib/shell/nav.js';
	import PrimaryMenu from '#lib/shell/PrimaryMenu.svelte';
	import ContextColumn from '#lib/shell/ContextColumn.svelte';
	import StepUpDialog from '#lib/StepUpDialog.svelte';
	import { stepUp } from '#lib/stepup.svelte.js';
	import { rowOpen } from '#lib/rowOpen.js';
	import type { LayoutProps } from './$types';

	let { data, children }: LayoutProps = $props();

	let manager = $derived(data.session.role === Role.OWNER || data.session.role === Role.ADMIN);
	let sections = $derived(navSections(manager));
	let section = $derived(sectionFor(sections, page.url.pathname));
	let user = $derived({ name: data.session.user?.name ?? '', email: data.session.user?.email ?? '' });

	async function logout() {
		await auth.logout({});
		await goto('/login');
	}
</script>

<svelte:document onclick={rowOpen} />

{#snippet profileHeader()}
	<div class="profile wb-me">
		<Avatar name={user.name} size={64} />
		<span class="profile__name">{user.name}</span>
		<span class="profile__email">{user.email}</span>
	</div>
{/snippet}

<div class={['shell', section.items.length === 0 && 'shell--bare']}>
	<PrimaryMenu {sections} active={section.id} {user} onlogout={logout} />
	{#if section.items.length > 0}
		<ContextColumn
			{section}
			current={itemFor(section, page.url.pathname)}
			header={section.id === 'profile' ? profileHeader : undefined}
		/>
	{/if}
	<main class="miso-scroll">
		<div class="work">{@render children()}</div>
	</main>
</div>
<!-- the dialog mounts per step-up, so each one asks the server for its factors afresh -->
{#if stepUp.open}<StepUpDialog />{/if}
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

	/* desktop from 1024px: the context column shows; below it only the rail stays */
	@media (min-width: 1024px) {
		.shell:not(.shell--bare) {
			grid-template-columns: 64px 260px 1fr;
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

	/* the whole row opens it (rowOpen) */
	:global(tbody tr:has(.row-name)) {
		cursor: pointer;
	}

	:global(.row-name:focus-visible) {
		outline: 2px solid var(--miso-line-accent);
		outline-offset: 2px;
		border-radius: var(--miso-radius-04);
	}
</style>
