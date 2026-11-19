<script lang="ts" generics="T">
	import type { Snippet } from 'svelte';
	import { EmptyState, Icon, Skeleton, type IconName } from '#lib/ui/index.js';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import type { Slot } from './dashboard';

	interface Props {
		title: string;
		icon: IconName;
		href: string;
		slot: Slot<T>;
		/** counter next to the title */
		count?: string;
		/** shown instead of the rows when the loaded value is an empty list */
		empty?: { icon: IconName; title: string };
		children: Snippet<[T]>;
	}

	let { title, icon, href, slot, count, empty, children }: Props = $props();
</script>

<section class="card">
	<header class="card__head">
		<span class="card__icon"><Icon name={icon} size={20} /></span>
		<h2 class="card__title">{title}</h2>
		{#if count}<span class="card__count">{count}</span>{/if}
		<a class="card__more" {href} aria-label={m.dashboard_open({ title })}><Icon name="chevron-right" size={16} /></a>
	</header>
	<div class="card__body">
		{#if slot.state === 'loading'}
			{#each [0, 1, 2] as i (i)}
				<div class="card__skel"><Skeleton variant="circle" width={36} height={36} /><Skeleton variant="text" lines={2} /></div>
			{/each}
		{:else if slot.state === 'error'}
			<LoadFailed onretry={slot.retry} />
		{:else if empty && Array.isArray(slot.value) && !slot.value.length}
			<EmptyState icon={empty.icon} title={empty.title} />
		{:else}
			{@render children(slot.value)}
		{/if}
	</div>
</section>

<style>
	/* adapted from the miso-ui demo dashboard .dbrd__block */
	.card {
		display: flex;
		flex-direction: column;
		min-width: 0;
		min-height: 320px;
		border-radius: var(--miso-radius-16);
		background: var(--miso-cont-top);
		box-shadow: var(--miso-shadow-cont-minor-outline);
		overflow: hidden;
	}
	.card__head {
		display: flex;
		align-items: center;
		gap: var(--miso-sp-08);
		padding: 10px 12px 12px 16px;
	}
	.card__icon {
		display: inline-flex;
		color: var(--miso-txt-secondary);
	}
	.card__title {
		margin: 0;
		font: var(--miso-font-ui-18-semi);
		color: var(--miso-txt-primary);
		white-space: nowrap;
	}
	.card__count {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 20px;
		height: 20px;
		padding: 0 4px;
		box-sizing: border-box;
		border-radius: var(--miso-radius-06);
		background: var(--miso-cont-low);
		font: var(--miso-font-ui-11-semi);
		color: var(--miso-txt-tertiary);
	}
	.card__more {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		margin-left: auto;
		border-radius: var(--miso-radius-08);
		color: var(--miso-txt-secondary);
	}
	.card__more:hover {
		background: var(--miso-hlt-g-hover);
	}
	.card__more:focus-visible {
		outline: 2px solid var(--wb-focus);
		outline-offset: -2px;
	}
	.card__body {
		display: flex;
		flex: 1;
		flex-direction: column;
		padding: 2px 0 8px;
	}
	.card__body :global(ul) {
		margin: 0;
		padding: 0;
	}
	.card__body > :global(.miso-empty) {
		flex: 1;
	}
	.card__skel {
		display: flex;
		align-items: center;
		gap: var(--miso-sp-12);
		padding: 8px 16px;
	}
</style>
