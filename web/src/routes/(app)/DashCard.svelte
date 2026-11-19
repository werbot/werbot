<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Icon } from '#lib/ui/index.js';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		title: string;
		href?: string;
		/** shown instead of the children */
		empty?: string;
		children?: Snippet;
	}

	let { title, href, empty, children }: Props = $props();
</script>

<section class="card">
	<header class="card__head">
		<h2 class="card__title">{title}</h2>
		{#if href}
			<a class="card__more" {href} aria-label={m.dashboard_open({ title })}><Icon name="chevron-right" size={16} /></a>
		{/if}
	</header>
	{#if empty}
		<p class="card__empty">{empty}</p>
	{:else}
		{@render children?.()}
	{/if}
</section>

<style>
	.card {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
		min-width: 0;
		padding: var(--miso-sp-16);
		border-radius: var(--miso-radius-08);
		background: var(--miso-cont-top);
		box-shadow: var(--miso-shadow-cont-minor-outline);
	}
	.card__head {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}
	.card__title {
		margin: 0;
		font: var(--miso-font-ui-14-semi);
		color: var(--miso-txt-primary);
	}
	.card__more {
		display: inline-flex;
		color: var(--miso-txt-secondary);
	}
	.card__empty {
		margin: 0;
		color: var(--miso-txt-secondary);
	}
</style>
